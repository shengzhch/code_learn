package learn_ccache

import (
	"testing"
	"fmt"
	"sync/atomic"
)

func Test_1(t *testing.T) {

	for i := 0; i < 10; i++ {
		//fmt.Println(int32(i) & (^(int32(i)) + 1))
		a := int32(i)
		fmt.Println(atomic.AddInt32(&a, -3))
		fmt.Println("a -- ", a, "  i-- ", i)
	}
}
