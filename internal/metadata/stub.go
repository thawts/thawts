package metadata

// StubProvider returns empty metadata. Replace with a platform-specific
// implementation (CGO + Accessibility API) when ready.
type StubProvider struct{}

func NewStubProvider() *StubProvider { return &StubProvider{} }

func (p *StubProvider) GetActiveWindowTitle() string { return "" }
func (p *StubProvider) GetActiveAppName() string     { return "" }
func (p *StubProvider) GetActiveURL() string         { return "" }
