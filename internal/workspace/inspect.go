package workspace

import "github.com/rainchan/webmux/internal/statefile"

func Inspect(dir string) error {
	_, err := statefile.ReadSnapshot[registry](dir, statefile.Repositories, validateRegistry)
	return err
}
func InspectGitOperations(dir string) error {
	_, err := statefile.ReadSnapshot[gitOperationLedger](dir, statefile.GitOperations, validateOperationLedger)
	return err
}
func (s *Service) StorageError() error {
	if s == nil {
		return statefile.ErrUnavailableStore()
	}
	return s.store.Err()
}
func (s *Service) OperationsStorageError() error {
	if s == nil {
		return statefile.ErrUnavailableStore()
	}
	return s.opsStore.Err()
}
