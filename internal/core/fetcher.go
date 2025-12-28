package core

// Fetcher is an interface for fetching data from URLs.
// This allows for easy mocking in tests.
type Fetcher interface {
	Fetch(url string) ([]byte, error)
}

// MockFetcher is a mock implementation of Fetcher for testing.
type MockFetcher struct {
	Responses map[string][]byte
	Errors    map[string]error
}

// NewMockFetcher creates a new MockFetcher.
func NewMockFetcher() *MockFetcher {
	return &MockFetcher{
		Responses: make(map[string][]byte),
		Errors:    make(map[string]error),
	}
}

// Fetch returns the mock response for the given URL.
func (m *MockFetcher) Fetch(url string) ([]byte, error) {
	if err, ok := m.Errors[url]; ok {
		return nil, err
	}
	if data, ok := m.Responses[url]; ok {
		return data, nil
	}
	return nil, nil
}

// AddResponse adds a mock response for a URL.
func (m *MockFetcher) AddResponse(url string, data []byte) {
	m.Responses[url] = data
}

// AddError adds a mock error for a URL.
func (m *MockFetcher) AddError(url string, err error) {
	m.Errors[url] = err
}
