package main

import (
	"errors"

	"github.com/Dyu-36/gotack/internal/contextseed"
)

// ContextMigrationPreview returns an immutable, hash-addressed migration
// candidate. The hashes and generation must be sent back when accepting.
func (a *App) ContextMigrationPreview() (contextseed.MigrationPreview, error) {
	if a.contextSeeder == nil {
		return contextseed.MigrationPreview{}, errors.New("context seeder is not initialized")
	}
	return a.contextSeeder.PreviewMigration()
}

func (a *App) AcceptContextMigration(req contextseed.AcceptMigrationRequest) (contextseed.MigrationStatus, error) {
	if a.contextSeeder == nil {
		return contextseed.MigrationStatus{}, errors.New("context seeder is not initialized")
	}
	status, err := a.contextSeeder.AcceptMigration(req)
	if err == nil {
		a.refreshCurrentContextSnapshot()
	}
	return status, err
}

func (a *App) RollbackContextMigration(req contextseed.RollbackMigrationRequest) (contextseed.MigrationStatus, error) {
	if a.contextSeeder == nil {
		return contextseed.MigrationStatus{}, errors.New("context seeder is not initialized")
	}
	status, err := a.contextSeeder.RollbackMigration(req)
	if err == nil {
		a.refreshCurrentContextSnapshot()
	}
	return status, err
}
