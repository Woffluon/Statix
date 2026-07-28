//go:build !linux && !unix

package metrics

func defaultStatfs(path string) (uint64, uint64, error) {
	return 0, 0, nil
}
