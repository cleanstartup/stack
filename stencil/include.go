package stencil

type include struct{}

func Include() include {
	return include{}
}

func (include) StackStencilInclude() {}
