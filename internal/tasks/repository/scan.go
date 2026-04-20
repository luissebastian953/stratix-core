// scan.go
package repository

type scanner interface {
	Scan(dest ...any) error
}
