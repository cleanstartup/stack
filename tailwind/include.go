package tailwind

type include struct{}

func Include() include {
	return include{}
}

func (include) StackTailwindInclude() {}
