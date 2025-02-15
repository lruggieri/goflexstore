package gormstore

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/jkaveri/goflexstore/converter"
	gormopscope "github.com/jkaveri/goflexstore/gorm/opscope"
	gormquery "github.com/jkaveri/goflexstore/gorm/query"
	gormutils "github.com/jkaveri/goflexstore/gorm/utils"
	"github.com/jkaveri/goflexstore/query"
	"github.com/jkaveri/goflexstore/store"
)

// New creates a new store
func New[Entity store.Entity[ID], DTO store.Entity[ID], ID comparable](
	opScope *gormopscope.TransactionScope,
	options ...Option[Entity, DTO, ID],
) *Store[Entity, DTO, ID] {
	s := &Store[Entity, DTO, ID]{
		OpScope:   opScope,
		BatchSize: 50,
	}

	for _, option := range options {
		option(s)
	}

	if s.Converter == nil {
		s.Converter = converter.NewReflect[Entity, DTO](nil)
	}

	if s.ScopeBuilder == nil {
		s.ScopeBuilder = gormquery.NewBuilder(
			gormquery.WithFieldToColMap(
				gormutils.FieldToColMap(new(DTO)),
			),
		)
	}

	return s
}

// Store is a gorm store
type Store[Entity store.Entity[ID], DTO store.Entity[ID], ID comparable] struct {
	OpScope      *gormopscope.TransactionScope
	Converter    converter.Converter[Entity, DTO, ID]
	ScopeBuilder *gormquery.ScopeBuilder
	BatchSize    int
}

// Get gets an entity
func (s *Store[Entity, DTO, ID]) Get(ctx context.Context, params ...query.Param) (Entity, error) {
	var (
		dto    DTO
		scopes = s.ScopeBuilder.Build(query.NewParams(params...))
	)

	if err := s.getTx(ctx).
		Scopes(scopes...).
		First(&dto).Error; err != nil {
		return *new(Entity), nil
	}

	return s.Converter.ToEntity(dto), nil
}

// List lists entities
func (s *Store[Entity, DTO, ID]) List(ctx context.Context, params ...query.Param) ([]Entity, error) {
	var (
		dtos   []DTO
		scopes = s.ScopeBuilder.Build(query.NewParams(params...))
	)

	if err := s.getTx(ctx).
		Scopes(scopes...).Find(&dtos).Error; err != nil {
		return nil, err
	}

	return converter.ToMany(dtos, s.Converter.ToEntity), nil
}

// Count counts entities
func (s *Store[Entity, DTO, ID]) Count(ctx context.Context, params ...query.Param) (int64, error) {
	var (
		count  int64
		scopes = s.ScopeBuilder.Build(query.NewParams(params...))
	)

	if err := s.getTx(ctx).
		Scopes(scopes...).
		Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// Exists checks if an entity exists
func (s *Store[Entity, DTO, ID]) Exists(ctx context.Context, params ...query.Param) (bool, error) {
	var (
		count  int64
		scopes = s.ScopeBuilder.Build(query.NewParams(params...))
	)

	if err := s.getTx(ctx).Scopes(scopes...).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// Delete deletes entities
func (s *Store[Entity, DTO, ID]) Create(ctx context.Context, entity Entity) (ID, error) {
	dto := s.Converter.ToDTO(entity)
	if err := s.getTx(ctx).Create(&dto).Error; err != nil {
		return *new(ID), err
	}

	return dto.GetID(), nil
}

// CreateMany batch create entities
//
// you can set BatchSize to control how many entities will be created in a batch
func (s *Store[Entity, DTO, ID]) CreateMany(ctx context.Context, entities []Entity) error {
	dtos := converter.ToMany(entities, s.Converter.ToDTO)
	batchSize := defaultValue(s.BatchSize, 50)

	return s.getTx(ctx).CreateInBatches(dtos, batchSize).Error
}

// Update updates an entity that including zero fields
func (s *Store[Entity, DTO, ID]) Update(ctx context.Context, entity Entity, params ...query.Param) error {
	dto := s.Converter.ToDTO(entity)
	id := dto.GetID()

	if id == *new(ID) && len(params) == 0 {
		return errors.New("id is required")
	}

	tx := s.getTx(ctx)

	if len(params) > 0 {
		scopes := s.ScopeBuilder.Build(query.NewParams(params...))
		tx = tx.Scopes(scopes...)
	}

	return tx.Save(&dto).Error
}

// PartialUpdate updates an entity partially that means only non-zero fields will be updated
func (s *Store[Entity, DTO, ID]) PartialUpdate(ctx context.Context, entity Entity, params ...query.Param) error {
	dto := s.Converter.ToDTO(entity)
	scopes := s.ScopeBuilder.Build(query.NewParams(params...))

	return s.getTx(ctx).Scopes(scopes...).Updates(dto).Error
}

func (s *Store[Entity, DTO, ID]) getTx(ctx context.Context) *gorm.DB {
	return s.OpScope.Tx(ctx).WithContext(ctx).Model(new(DTO))
}
