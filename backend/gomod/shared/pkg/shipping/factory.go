package shipping

import (
	"fmt"
	"sync"
)

// ShippingFactory quản lý và cung cấp các ShippingProvider (Strategy Pattern)
type ShippingFactory struct {
	mu        sync.RWMutex
	providers map[string]ShippingProvider
}

// NewShippingFactory khởi tạo factory với danh sách các provider ban đầu
func NewShippingFactory(providers ...ShippingProvider) *ShippingFactory {
	f := &ShippingFactory{
		providers: make(map[string]ShippingProvider),
	}
	for _, p := range providers {
		if p != nil {
			f.providers[p.Name()] = p
		}
	}
	return f
}

// Register đăng ký thêm một provider mới vào factory
func (f *ShippingFactory) Register(provider ShippingProvider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[provider.Name()] = provider
}

// GetProvider lấy provider theo tên (ví dụ: "ghn")
func (f *ShippingFactory) GetProvider(name string) (ShippingProvider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	p, exists := f.providers[name]
	if !exists {
		return nil, fmt.Errorf("shipping provider '%s' is not supported", name)
	}
	return p, nil
}
