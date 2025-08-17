package catalogs

type Catalog string

const (
	Embedded Catalog = "embedded"
	Files    Catalog = "files"
	Memory   Catalog = "memory"
)

func (c Catalog) String() string {
	return string(c)
}
