package dict

type Province int

const (
	JXYD Province = iota
	GXYD
	XJYD
	AHYD
)

func (province Province) String() string {
	switch province {
	case JXYD:
		return "jxyd"
	case GXYD:
		return "gxyd"
	case XJYD:
		return "xjyd"
	case AHYD:
		return "ahyd"
	default:
		return "default"
	}
}
