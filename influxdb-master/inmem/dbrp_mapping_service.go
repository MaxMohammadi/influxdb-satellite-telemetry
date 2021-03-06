package inmem

import (
	"context"
	"fmt"
	"path"

	"github.com/influxdata/influxdb/v2/kit/platform/errors"

	"github.com/influxdata/influxdb/v2"
)

var (
	errDBRPMappingNotFound = &errors.Error{
		Code: errors.ENotFound,
		Msg:  "dbrp mapping not found",
	}
)

func encodeDBRPMappingKey(cluster, db, rp string) string {
	return path.Join(cluster, db, rp)
}

func (s *Service) loadDBRPMapping(ctx context.Context, cluster, db, rp string) (*influxdb.DBRPMapping, error) {
	i, ok := s.dbrpMappingKV.Load(encodeDBRPMappingKey(cluster, db, rp))
	if !ok {
		return nil, errDBRPMappingNotFound
	}

	m, ok := i.(influxdb.DBRPMapping)
	if !ok {
		return nil, fmt.Errorf("type %T is not a dbrp mapping", i)
	}

	return &m, nil
}

// FindBy returns a single dbrp mapping by cluster, db and rp.
func (s *Service) FindBy(ctx context.Context, cluster, db, rp string) (*influxdb.DBRPMapping, error) {
	return s.loadDBRPMapping(ctx, cluster, db, rp)
}

func (s *Service) forEachDBRPMapping(ctx context.Context, fn func(m *influxdb.DBRPMapping) bool) error {
	var err error
	s.dbrpMappingKV.Range(func(k, v interface{}) bool {
		m, ok := v.(influxdb.DBRPMapping)
		if !ok {
			err = fmt.Errorf("type %T is not a dbrp mapping", v)
			return false
		}
		return fn(&m)
	})

	return err
}

func (s *Service) filterDBRPMappings(ctx context.Context, fn func(m *influxdb.DBRPMapping) bool) ([]*influxdb.DBRPMapping, error) {
	mappings := []*influxdb.DBRPMapping{}
	err := s.forEachDBRPMapping(ctx, func(m *influxdb.DBRPMapping) bool {
		if fn(m) {
			mappings = append(mappings, m)
		}
		return true
	})

	if err != nil {
		return nil, err
	}

	return mappings, nil
}

// Find returns the first dbrp mapping that matches filter.
func (s *Service) Find(ctx context.Context, filter influxdb.DBRPMappingFilter) (*influxdb.DBRPMapping, error) {
	if filter.Cluster == nil && filter.Database == nil && filter.RetentionPolicy == nil {
		return nil, &errors.Error{
			Code: errors.EInvalid,
			Msg:  "no filter parameters provided",
		}
	}

	// filter by dbrpMapping id
	if filter.Cluster != nil && filter.Database != nil && filter.RetentionPolicy != nil {
		return s.FindBy(ctx, *filter.Cluster, *filter.Database, *filter.RetentionPolicy)
	}

	mappings, n, err := s.FindMany(ctx, filter)
	if err != nil {
		return nil, err
	}

	if n < 1 {
		return nil, errDBRPMappingNotFound
	}

	return mappings[0], nil
}

// FindMany returns a list of dbrpMappings that match filter and the total count of matching dbrp mappings.
// Additional options provide pagination & sorting.
func (s *Service) FindMany(ctx context.Context, filter influxdb.DBRPMappingFilter, opt ...influxdb.FindOptions) ([]*influxdb.DBRPMapping, int, error) {
	// filter by dbrpMapping id
	if filter.Cluster != nil && filter.Database != nil && filter.RetentionPolicy != nil {
		m, err := s.FindBy(ctx, *filter.Cluster, *filter.Database, *filter.RetentionPolicy)
		if err != nil {
			return nil, 0, err
		}
		return []*influxdb.DBRPMapping{m}, 1, nil
	}

	filterFunc := func(mapping *influxdb.DBRPMapping) bool {
		return (filter.Cluster == nil || (*filter.Cluster) == mapping.Cluster) &&
			(filter.Database == nil || (*filter.Database) == mapping.Database) &&
			(filter.RetentionPolicy == nil || (*filter.RetentionPolicy) == mapping.RetentionPolicy) &&
			(filter.Default == nil || (*filter.Default) == mapping.Default)
	}

	mappings, err := s.filterDBRPMappings(ctx, filterFunc)
	if err != nil {
		return nil, 0, err
	}

	return mappings, len(mappings), nil
}

// Create creates a new dbrp mapping.
func (s *Service) Create(ctx context.Context, m *influxdb.DBRPMapping) error {
	if err := m.Validate(); err != nil {
		return nil
	}
	existing, err := s.loadDBRPMapping(ctx, m.Cluster, m.Database, m.RetentionPolicy)
	if err != nil {
		if err == errDBRPMappingNotFound {
			return s.PutDBRPMapping(ctx, m)
		}
		return err
	}

	if !existing.Equal(m) {
		return &errors.Error{
			Code: errors.EConflict,
			Msg:  "dbrp mapping already exists",
		}
	}

	return s.PutDBRPMapping(ctx, m)
}

// PutDBRPMapping sets dbrpMapping with the current ID.
func (s *Service) PutDBRPMapping(ctx context.Context, m *influxdb.DBRPMapping) error {
	k := encodeDBRPMappingKey(m.Cluster, m.Database, m.RetentionPolicy)
	s.dbrpMappingKV.Store(k, *m)
	return nil
}

// Delete removes a dbrp mapping
func (s *Service) Delete(ctx context.Context, cluster, db, rp string) error {
	s.dbrpMappingKV.Delete(encodeDBRPMappingKey(cluster, db, rp))
	return nil
}
