package a

type mixed struct{}

func (m *mixed) one() {}

func (m mixed) two() {} // want `methods of mixed mix pointer and value receivers`

type pointerOnly struct{}

func (p *pointerOnly) one() {}

func (p *pointerOnly) two() {}

type valueOnly struct{}

func (v valueOnly) one() {}

func (v valueOnly) two() {}

type many struct{}

func (m *many) one() {}

func (m many) two() {} // want `methods of many mix pointer and value receivers`

func (m *many) three() {}
