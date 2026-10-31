// internal/utils/profanity_filter.go
package utils

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

// Pre-compiled regex để tối ưu hiệu năng, tránh compile lại mỗi lần gọi.
var (
	// Bắt URL/link spam — mở rộng so với bản cũ:
	// - Bắt domain.tld tổng quát (không chỉ hardcode "bit.ly"/"t.me"), vì kẻ
	//   spam chỉ cần đổi sang tinyurl.com/zalo.me/domain riêng là né được
	//   filter cũ hoàn toàn.
	// - Bắt cả biến thể "hxxp" (cách né phổ biến để tránh regex http/https).
	urlPattern = regexp.MustCompile(`(?i)\b((h[tx][tx]ps?:?/?/?)|www\.)?[a-z0-9-]+\.(com|net|org|vn|me|ly|xyz|top|click|info|gg|io|co|tk|cc|ru|shop|store|page|link)\b`)

	// Bắt số điện thoại VN (10-11 số, có thể chèn dấu chấm/gạch/khoảng trắng).
	phonePattern = regexp.MustCompile(`(0|\+84)[\s\.\-_]*[1-9][0-9]{1,2}[\s\.\-_]*[0-9]{3}[\s\.\-_]*[0-9]{3}`)

	// Loại bỏ ký tự vô hình (zero-width space...) thường dùng để né filter.
	invisibleCharsPattern = regexp.MustCompile(`[\x{200B}\x{200C}\x{200D}\x{FEFF}]`)

	// Các cụm từ cấm nhiều từ (đã transliterate về không dấu, khớp CHÍNH XÁC
	// theo word boundary, không dùng substring matching thô để tránh false
	// positive).
	badPhrases = []string{
		"lua dao", "cho de",
	}

	// Từ cấm đơn — CHỈ chứa từ đã transliterate về không dấu (không còn ký
	// tự có dấu như bản cũ), vì bước chuẩn hóa bên dưới luôn chuyển input về
	// không dấu trước khi so khớp. Giữ "đụ" (có dấu) trong danh sách này ở
	// bản cũ là bug — nó sẽ KHÔNG BAO GIỜ khớp vì input sau chuẩn hóa không
	// còn dấu, còn từ trong danh sách vẫn có dấu.
	badWords = []string{
		"dm", "dcm", "cl", "vcl", "vl", "du", "dit", "cac", "lon", "buoi",
		"fuck", "shit", "bitch", "scam",
	}

	// wordSplitPattern dùng để tách text thành các "token" (từ) — coi mọi
	// ký tự không phải chữ/số là ranh giới từ. Nhờ tách theo token thay vì
	// so khớp thô trên toàn chuỗi, ta tránh được việc 1 từ hợp lệ dài chứa
	// tình cờ 1 từ cấm ngắn làm substring (vd "credit" chứa "dit").
	wordSplitPattern = regexp.MustCompile(`[^a-z0-9]+`)
)

// ValidateReviewContent kiểm tra tiêu đề và nội dung đánh giá.
// Trả về lỗi rõ ràng nếu phát hiện từ ngữ thô tục, link spam hoặc SĐT lừa đảo.
func ValidateReviewContent(title, comment *string) error {
	texts := make([]string, 0, 2)
	if title != nil && *title != "" {
		texts = append(texts, *title)
	}
	if comment != nil && *comment != "" {
		texts = append(texts, *comment)
	}

	for _, text := range texts {
		cleaned := invisibleCharsPattern.ReplaceAllString(text, "")

		// 1. Chặn link rác — chuẩn hóa các biến thể né "." phổ biến trước
		// khi match (vd "bit[.]ly", "bit . ly").
		urlCandidate := normalizeDotEvasion(cleaned)
		if urlPattern.MatchString(urlCandidate) {
			return errors.New("review content cannot contain external links or URLs")
		}

		// 2. Chặn số điện thoại quảng cáo
		if phonePattern.MatchString(cleaned) {
			return errors.New("review content cannot contain phone numbers")
		}

		// 3. Chuẩn hóa về chữ thường KHÔNG DẤU (transliterate, không phải
		// xóa ký tự có dấu như Slugify cũ) để so khớp từ cấm.
		normalized := removeVietnameseDiacritics(strings.ToLower(cleaned))

		if containsBadPhrase(normalized) {
			return errors.New("review content contains inappropriate or prohibited language")
		}

		if containsBadWord(normalized) {
			return errors.New("review content contains inappropriate or prohibited language")
		}
	}

	return nil
}

// containsBadPhrase kiểm tra cụm nhiều từ (vd "lua dao") — chuẩn hóa
// khoảng trắng liên tiếp về 1 dấu cách trước khi so khớp theo word boundary,
// tránh việc chèn nhiều khoảng trắng ("lua    dao") để né filter.
func containsBadPhrase(normalized string) bool {
	collapsed := strings.Join(strings.Fields(normalized), " ")
	for _, phrase := range badPhrases {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(phrase) + `\b`).MatchString(collapsed) {
			return true
		}
	}
	return false
}

// containsBadWord kiểm tra từng TOKEN (từ đã tách theo ranh giới không phải
// chữ/số) khớp CHÍNH XÁC với 1 từ cấm — không dùng strings.Contains thô lên
// toàn bộ chuỗi (đây chính là bug false-positive của bản cũ: "credit" chứa
// "dit", "class"/"clock" chứa "cl", "cactus" chứa "cac"... đều bị chặn nhầm
// nếu chỉ check substring).
//
// Đồng thời chống né tránh kiểu chèn ký tự giữa các chữ cái ("d.m", "v c l",
// "vcllll") bằng cách thử so khớp cả token gốc LẪN phiên bản đã "nén" các
// ký tự lặp liên tiếp về 1 ký tự (loại "vcllll" -> "vcl").
func containsBadWord(normalized string) bool {
	tokens := wordSplitPattern.Split(normalized, -1)

	for _, token := range tokens {
		if token == "" {
			continue
		}
		compact := collapseRepeatedRunes(token)

		for _, bad := range badWords {
			if token == bad || compact == bad {
				return true
			}
		}
	}
	return false
}

// collapseRepeatedRunes nén các ký tự lặp liên tiếp (>=3 lần) về đúng 1 lần
// — bắt kỹ thuật né tránh phổ biến kiểu gõ lặp ký tự để phá vỡ so khớp chính
// xác ("vcllll", "dmmmm") mà vẫn giữ nguyên các từ hợp lệ có lặp tự nhiên tối
// đa 2 ký tự liên tiếp (vd "look", "class" không bị ảnh hưởng).
func collapseRepeatedRunes(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		count := 1
		for i+count < len(runes) && runes[i+count] == runes[i] {
			count++
		}
		if count >= 3 {
			b.WriteRune(runes[i]) // nén về 1 ký tự
			i += count - 1
		} else {
			for j := 0; j < count; j++ {
				b.WriteRune(runes[i])
			}
			i += count - 1
		}
	}
	return b.String()
}

// normalizeDotEvasion thay các biến thể né dấu "." phổ biến về lại "." thật,
// giúp urlPattern bắt được cả link bị cố tình viết lệch chuẩn.
func normalizeDotEvasion(s string) string {
	replacer := strings.NewReplacer(
		"[.]", ".",
		"(.)", ".",
		" dot ", ".",
		" . ", ".",
	)
	return replacer.Replace(s)
}

// removeVietnameseDiacritics chuyển ký tự tiếng Việt có dấu về dạng không
// dấu tương ứng (á→a, đ→d, ê→e...), KHÔNG xóa hẳn ký tự như Slugify —
// đây chính là lỗi khiến badWords có dấu ("đụ") không bao giờ khớp được ở
// bản cũ, vì Slugify chỉ giữ [a-z0-9-] và loại bỏ hoàn toàn ký tự có dấu
// thay vì chuyển đổi chúng.
func removeVietnameseDiacritics(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case 'đ':
			return 'd'
		case 'Đ':
			return 'D'
		}
		return r
	}, s)

	var b strings.Builder
	for _, r := range s {
		if replacement, ok := diacriticMap[r]; ok {
			b.WriteRune(replacement)
			continue
		}
		if unicode.Is(unicode.Mn, r) {
			// Bỏ qua dấu tổ hợp còn sót lại (nếu input đã ở dạng NFD)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// diacriticMap ánh xạ trực tiếp từng ký tự tiếng Việt có dấu (dạng precomposed,
// phổ biến nhất khi nhận từ JSON/trình duyệt) sang ký tự Latin gốc.
var diacriticMap = map[rune]rune{
	'à': 'a', 'á': 'a', 'ả': 'a', 'ã': 'a', 'ạ': 'a',
	'ă': 'a', 'ằ': 'a', 'ắ': 'a', 'ẳ': 'a', 'ẵ': 'a', 'ặ': 'a',
	'â': 'a', 'ầ': 'a', 'ấ': 'a', 'ẩ': 'a', 'ẫ': 'a', 'ậ': 'a',
	'è': 'e', 'é': 'e', 'ẻ': 'e', 'ẽ': 'e', 'ẹ': 'e',
	'ê': 'e', 'ề': 'e', 'ế': 'e', 'ể': 'e', 'ễ': 'e', 'ệ': 'e',
	'ì': 'i', 'í': 'i', 'ỉ': 'i', 'ĩ': 'i', 'ị': 'i',
	'ò': 'o', 'ó': 'o', 'ỏ': 'o', 'õ': 'o', 'ọ': 'o',
	'ô': 'o', 'ồ': 'o', 'ố': 'o', 'ổ': 'o', 'ỗ': 'o', 'ộ': 'o',
	'ơ': 'o', 'ờ': 'o', 'ớ': 'o', 'ở': 'o', 'ỡ': 'o', 'ợ': 'o',
	'ù': 'u', 'ú': 'u', 'ủ': 'u', 'ũ': 'u', 'ụ': 'u',
	'ư': 'u', 'ừ': 'u', 'ứ': 'u', 'ử': 'u', 'ữ': 'u', 'ự': 'u',
	'ỳ': 'y', 'ý': 'y', 'ỷ': 'y', 'ỹ': 'y', 'ỵ': 'y',
	'À': 'A', 'Á': 'A', 'Ả': 'A', 'Ã': 'A', 'Ạ': 'A',
	'Ă': 'A', 'Ằ': 'A', 'Ắ': 'A', 'Ẳ': 'A', 'Ẵ': 'A', 'Ặ': 'A',
	'Â': 'A', 'Ầ': 'A', 'Ấ': 'A', 'Ẩ': 'A', 'Ẫ': 'A', 'Ậ': 'A',
	'È': 'E', 'É': 'E', 'Ẻ': 'E', 'Ẽ': 'E', 'Ẹ': 'E',
	'Ê': 'E', 'Ề': 'E', 'Ế': 'E', 'Ể': 'E', 'Ễ': 'E', 'Ệ': 'E',
	'Ì': 'I', 'Í': 'I', 'Ỉ': 'I', 'Ĩ': 'I', 'Ị': 'I',
	'Ò': 'O', 'Ó': 'O', 'Ỏ': 'O', 'Õ': 'O', 'Ọ': 'O',
	'Ô': 'O', 'Ồ': 'O', 'Ố': 'O', 'Ổ': 'O', 'Ỗ': 'O', 'Ộ': 'O',
	'Ơ': 'O', 'Ờ': 'O', 'Ớ': 'O', 'Ở': 'O', 'Ỡ': 'O', 'Ợ': 'O',
	'Ù': 'U', 'Ú': 'U', 'Ủ': 'U', 'Ũ': 'U', 'Ụ': 'U',
	'Ư': 'U', 'Ừ': 'U', 'Ứ': 'U', 'Ử': 'U', 'Ữ': 'U', 'Ự': 'U',
	'Ỳ': 'Y', 'Ý': 'Y', 'Ỷ': 'Y', 'Ỹ': 'Y', 'Ỵ': 'Y',
}
