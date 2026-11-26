package a // want `lists TestListedNoReason without a reason` `lists TestGone, which the package does not declare`

import (
	"testing"
	"testing/synctest"
)

func TestDirect(t *testing.T) { t.Parallel() }

func TestViaHelper(t *testing.T) { newEnv(t) }

func TestViaTwoHelpers(t *testing.T) { outer(t, 1) }

func TestViaCallback(t *testing.T) { eachBackend(t, func(t *testing.T) {}) }

func TestSubtestOnly(t *testing.T) { // want `TestSubtestOnly does not call t.Parallel`
	t.Run("x", func(t *testing.T) { t.Parallel() })
}

func TestOtherT(t *testing.T) { // want `TestOtherT does not call t.Parallel`
	var other *testing.T
	other.Parallel()
}

func TestHelperOnOtherT(t *testing.T) { // want `TestHelperOnOtherT does not call t.Parallel`
	var other *testing.T
	newEnv(other)
}

func TestHelperThatDoesNotParallelize(t *testing.T) { // want `TestHelperThatDoesNotParallelize does not call t.Parallel`
	plain(t)
}

func TestInBubble(t *testing.T) { // want `TestInBubble does not call t.Parallel`
	synctest.Test(t, func(t *testing.T) {})
}

func TestSerialListed(t *testing.T) { t.Setenv("A", "b") }

func TestListedNoReason(t *testing.T) {}

func TestListedButParallel(t *testing.T) { // want `TestListedButParallel calls t.Parallel but`
	t.Parallel()
}

func TestMain(m *testing.M) { m.Run() }

func Testlower(t *testing.T) {}

func newEnv(t *testing.T) { t.Parallel() }

func outer(t *testing.T, n int) { newEnv(t) }

func eachBackend(t *testing.T, fn func(t *testing.T)) {
	t.Parallel()
	t.Run("x", fn)
}

func plain(t *testing.T) { t.Helper() }
