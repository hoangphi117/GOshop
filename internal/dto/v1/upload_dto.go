package v1dto

type UploadResponseDTO struct {
	FileName  string `json:"file_name"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	MediaType string `json:"media_type"`
}
