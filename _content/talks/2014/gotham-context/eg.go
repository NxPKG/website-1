// +build ignore,OMIT

package P

import (
	"xpkg"
	"ypkg"

	"github.com/khulnasoft-lab/godep/net/context"
)

func before(x xpkg.X, y ypkg.Y) error { // HL
	return x.M(y)
}

func after(x xpkg.X, y ypkg.Y) error { // HL
	return x.MContext(context.TODO(), y)
}
