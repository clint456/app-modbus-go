package bootstarp

type BootStarp struct {
}

func NewBootStarp() *BootStarp {
	return &BootStarp{}
}

func (b *BootStarp) Initialize() error {
	return nil
}

func (b *BootStarp) Start() error {
	return nil
}

func (b *BootStarp) Stop() error {
	return nil
}
