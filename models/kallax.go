// IMPORTANT! This is auto generated code by https://github.com/src-d/go-kallax
// Please, do not touch the code below, and if you do, do it under your own
// risk. Take into account that all the code you write here will be completely
// erased from earth the next time you generate the kallax models.
package models

import (
	"database/sql"
	"fmt"
	"time"

	"gopkg.in/src-d/go-kallax.v1"
	"gopkg.in/src-d/go-kallax.v1/types"
)

var _ types.SQLType
var _ fmt.Formatter

type modelSaveFunc func(*kallax.Store) error

// NewInstance returns a new instance of Instance.
func NewInstance() (record *Instance) {
	return newInstance()
}

// GetID returns the primary key of the model.
func (r *Instance) GetID() kallax.Identifier {
	return (*kallax.ULID)(&r.ID)
}

// ColumnAddress returns the pointer to the value of the given column.
func (r *Instance) ColumnAddress(col string) (interface{}, error) {
	switch col {
	case "id":
		return (*kallax.ULID)(&r.ID), nil
	case "created_at":
		return &r.Timestamps.CreatedAt, nil
	case "updated_at":
		return &r.Timestamps.UpdatedAt, nil
	case "spec_id":
		return types.Nullable(kallax.VirtualColumn("spec_id", r, new(kallax.ULID))), nil
	case "image_id":
		return &r.ImageID, nil
	case "container_id":
		return &r.ContainerID, nil
	case "expires_at":
		return types.Nullable(&r.ExpiresAt), nil
	case "active":
		return &r.Active, nil
	case "cleaned":
		return &r.Cleaned, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in Instance: %s", col)
	}
}

// Value returns the value of the given column.
func (r *Instance) Value(col string) (interface{}, error) {
	switch col {
	case "id":
		return r.ID, nil
	case "created_at":
		return r.Timestamps.CreatedAt, nil
	case "updated_at":
		return r.Timestamps.UpdatedAt, nil
	case "spec_id":
		v := r.Model.VirtualColumn(col)
		if v == nil {
			return nil, kallax.ErrEmptyVirtualColumn
		}
		return v, nil
	case "image_id":
		return r.ImageID, nil
	case "container_id":
		return r.ContainerID, nil
	case "expires_at":
		if r.ExpiresAt == (*time.Time)(nil) {
			return nil, nil
		}
		return r.ExpiresAt, nil
	case "active":
		return r.Active, nil
	case "cleaned":
		return r.Cleaned, nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in Instance: %s", col)
	}
}

// NewRelationshipRecord returns a new record for the relatiobship in the given
// field.
func (r *Instance) NewRelationshipRecord(field string) (kallax.Record, error) {
	switch field {
	case "Spec":
		return new(Spec), nil

	}
	return nil, fmt.Errorf("kallax: model Instance has no relationship %s", field)
}

// SetRelationship sets the given relationship in the given field.
func (r *Instance) SetRelationship(field string, rel interface{}) error {
	switch field {
	case "Spec":
		val, ok := rel.(*Spec)
		if !ok {
			return fmt.Errorf("kallax: record of type %t can't be assigned to relationship Spec", rel)
		}
		if !val.GetID().IsEmpty() {
			r.Spec = val
		}

		return nil

	}
	return fmt.Errorf("kallax: model Instance has no relationship %s", field)
}

// InstanceStore is the entity to access the records of the type Instance
// in the database.
type InstanceStore struct {
	*kallax.Store
}

// NewInstanceStore creates a new instance of InstanceStore
// using a SQL database.
func NewInstanceStore(db *sql.DB) *InstanceStore {
	return &InstanceStore{kallax.NewStore(db)}
}

// GenericStore returns the generic store of this store.
func (s *InstanceStore) GenericStore() *kallax.Store {
	return s.Store
}

// SetGenericStore changes the generic store of this store.
func (s *InstanceStore) SetGenericStore(store *kallax.Store) {
	s.Store = store
}

// Debug returns a new store that will print all SQL statements to stdout using
// the log.Printf function.
func (s *InstanceStore) Debug() *InstanceStore {
	return &InstanceStore{s.Store.Debug()}
}

// DebugWith returns a new store that will print all SQL statements using the
// given logger function.
func (s *InstanceStore) DebugWith(logger kallax.LoggerFunc) *InstanceStore {
	return &InstanceStore{s.Store.DebugWith(logger)}
}

func (s *InstanceStore) inverseRecords(record *Instance) []modelSaveFunc {
	var result []modelSaveFunc

	if record.Spec != nil && !record.Spec.IsSaving() {
		record.AddVirtualColumn("spec_id", record.Spec.GetID())
		result = append(result, func(store *kallax.Store) error {
			_, err := (&SpecStore{store}).Save(record.Spec)
			return err
		})
	}

	return result
}

// Insert inserts a Instance in the database. A non-persisted object is
// required for this operation.
func (s *InstanceStore) Insert(record *Instance) error {
	record.SetSaving(true)
	defer record.SetSaving(false)

	record.CreatedAt = record.CreatedAt.Truncate(time.Microsecond)
	record.UpdatedAt = record.UpdatedAt.Truncate(time.Microsecond)
	if record.ExpiresAt != nil {
		record.ExpiresAt = func(t time.Time) *time.Time { return &t }(record.ExpiresAt.Truncate(time.Microsecond))
	}

	if err := record.BeforeSave(); err != nil {
		return err
	}

	inverseRecords := s.inverseRecords(record)

	if len(inverseRecords) > 0 {
		return s.Store.Transaction(func(s *kallax.Store) error {
			for _, r := range inverseRecords {
				if err := r(s); err != nil {
					return err
				}
			}

			if err := s.Insert(Schema.Instance.BaseSchema, record); err != nil {
				return err
			}

			return nil
		})
	}

	return s.Store.Insert(Schema.Instance.BaseSchema, record)
}

// Update updates the given record on the database. If the columns are given,
// only these columns will be updated. Otherwise all of them will be.
// Be very careful with this, as you will have a potentially different object
// in memory but not on the database.
// Only writable records can be updated. Writable objects are those that have
// been just inserted or retrieved using a query with no custom select fields.
func (s *InstanceStore) Update(record *Instance, cols ...kallax.SchemaField) (updated int64, err error) {
	record.CreatedAt = record.CreatedAt.Truncate(time.Microsecond)
	record.UpdatedAt = record.UpdatedAt.Truncate(time.Microsecond)
	if record.ExpiresAt != nil {
		record.ExpiresAt = func(t time.Time) *time.Time { return &t }(record.ExpiresAt.Truncate(time.Microsecond))
	}

	record.SetSaving(true)
	defer record.SetSaving(false)

	if err := record.BeforeSave(); err != nil {
		return 0, err
	}

	inverseRecords := s.inverseRecords(record)

	if len(inverseRecords) > 0 {
		err = s.Store.Transaction(func(s *kallax.Store) error {
			for _, r := range inverseRecords {
				if err := r(s); err != nil {
					return err
				}
			}

			updated, err = s.Update(Schema.Instance.BaseSchema, record, cols...)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return 0, err
		}

		return updated, nil
	}

	return s.Store.Update(Schema.Instance.BaseSchema, record, cols...)
}

// Save inserts the object if the record is not persisted, otherwise it updates
// it. Same rules of Update and Insert apply depending on the case.
func (s *InstanceStore) Save(record *Instance) (updated bool, err error) {
	if !record.IsPersisted() {
		return false, s.Insert(record)
	}

	rowsUpdated, err := s.Update(record)
	if err != nil {
		return false, err
	}

	return rowsUpdated > 0, nil
}

// Delete removes the given record from the database.
func (s *InstanceStore) Delete(record *Instance) error {
	return s.Store.Delete(Schema.Instance.BaseSchema, record)
}

// Find returns the set of results for the given query.
func (s *InstanceStore) Find(q *InstanceQuery) (*InstanceResultSet, error) {
	rs, err := s.Store.Find(q)
	if err != nil {
		return nil, err
	}

	return NewInstanceResultSet(rs), nil
}

// MustFind returns the set of results for the given query, but panics if there
// is any error.
func (s *InstanceStore) MustFind(q *InstanceQuery) *InstanceResultSet {
	return NewInstanceResultSet(s.Store.MustFind(q))
}

// Count returns the number of rows that would be retrieved with the given
// query.
func (s *InstanceStore) Count(q *InstanceQuery) (int64, error) {
	return s.Store.Count(q)
}

// MustCount returns the number of rows that would be retrieved with the given
// query, but panics if there is an error.
func (s *InstanceStore) MustCount(q *InstanceQuery) int64 {
	return s.Store.MustCount(q)
}

// FindOne returns the first row returned by the given query.
// `ErrNotFound` is returned if there are no results.
func (s *InstanceStore) FindOne(q *InstanceQuery) (*Instance, error) {
	q.Limit(1)
	q.Offset(0)
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	if !rs.Next() {
		return nil, kallax.ErrNotFound
	}

	record, err := rs.Get()
	if err != nil {
		return nil, err
	}

	if err := rs.Close(); err != nil {
		return nil, err
	}

	return record, nil
}

// FindAll returns a list of all the rows returned by the given query.
func (s *InstanceStore) FindAll(q *InstanceQuery) ([]*Instance, error) {
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	return rs.All()
}

// MustFindOne returns the first row retrieved by the given query. It panics
// if there is an error or if there are no rows.
func (s *InstanceStore) MustFindOne(q *InstanceQuery) *Instance {
	record, err := s.FindOne(q)
	if err != nil {
		panic(err)
	}
	return record
}

// Reload refreshes the Instance with the data in the database and
// makes it writable.
func (s *InstanceStore) Reload(record *Instance) error {
	return s.Store.Reload(Schema.Instance.BaseSchema, record)
}

// Transaction executes the given callback in a transaction and rollbacks if
// an error is returned.
// The transaction is only open in the store passed as a parameter to the
// callback.
func (s *InstanceStore) Transaction(callback func(*InstanceStore) error) error {
	if callback == nil {
		return kallax.ErrInvalidTxCallback
	}

	return s.Store.Transaction(func(store *kallax.Store) error {
		return callback(&InstanceStore{store})
	})
}

// InstanceQuery is the object used to create queries for the Instance
// entity.
type InstanceQuery struct {
	*kallax.BaseQuery
}

// NewInstanceQuery returns a new instance of InstanceQuery.
func NewInstanceQuery() *InstanceQuery {
	return &InstanceQuery{
		BaseQuery: kallax.NewBaseQuery(Schema.Instance.BaseSchema),
	}
}

// Select adds columns to select in the query.
func (q *InstanceQuery) Select(columns ...kallax.SchemaField) *InstanceQuery {
	if len(columns) == 0 {
		return q
	}
	q.BaseQuery.Select(columns...)
	return q
}

// SelectNot excludes columns from being selected in the query.
func (q *InstanceQuery) SelectNot(columns ...kallax.SchemaField) *InstanceQuery {
	q.BaseQuery.SelectNot(columns...)
	return q
}

// Copy returns a new identical copy of the query. Remember queries are mutable
// so make a copy any time you need to reuse them.
func (q *InstanceQuery) Copy() *InstanceQuery {
	return &InstanceQuery{
		BaseQuery: q.BaseQuery.Copy(),
	}
}

// Order adds order clauses to the query for the given columns.
func (q *InstanceQuery) Order(cols ...kallax.ColumnOrder) *InstanceQuery {
	q.BaseQuery.Order(cols...)
	return q
}

// BatchSize sets the number of items to fetch per batch when there are 1:N
// relationships selected in the query.
func (q *InstanceQuery) BatchSize(size uint64) *InstanceQuery {
	q.BaseQuery.BatchSize(size)
	return q
}

// Limit sets the max number of items to retrieve.
func (q *InstanceQuery) Limit(n uint64) *InstanceQuery {
	q.BaseQuery.Limit(n)
	return q
}

// Offset sets the number of items to skip from the result set of items.
func (q *InstanceQuery) Offset(n uint64) *InstanceQuery {
	q.BaseQuery.Offset(n)
	return q
}

// Where adds a condition to the query. All conditions added are concatenated
// using a logical AND.
func (q *InstanceQuery) Where(cond kallax.Condition) *InstanceQuery {
	q.BaseQuery.Where(cond)
	return q
}

func (q *InstanceQuery) WithSpec() *InstanceQuery {
	q.AddRelation(Schema.Spec.BaseSchema, "Spec", kallax.OneToOne, nil)
	return q
}

// FindByID adds a new filter to the query that will require that
// the ID property is equal to one of the passed values; if no passed values,
// it will do nothing.
func (q *InstanceQuery) FindByID(v ...kallax.ULID) *InstanceQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.In(Schema.Instance.ID, values...))
}

// FindByCreatedAt adds a new filter to the query that will require that
// the CreatedAt property is equal to the passed value.
func (q *InstanceQuery) FindByCreatedAt(cond kallax.ScalarCond, v time.Time) *InstanceQuery {
	return q.Where(cond(Schema.Instance.CreatedAt, v))
}

// FindByUpdatedAt adds a new filter to the query that will require that
// the UpdatedAt property is equal to the passed value.
func (q *InstanceQuery) FindByUpdatedAt(cond kallax.ScalarCond, v time.Time) *InstanceQuery {
	return q.Where(cond(Schema.Instance.UpdatedAt, v))
}

// FindBySpec adds a new filter to the query that will require that
// the foreign key of Spec is equal to the passed value.
func (q *InstanceQuery) FindBySpec(v kallax.ULID) *InstanceQuery {
	return q.Where(kallax.Eq(Schema.Instance.SpecFK, v))
}

// FindByImageID adds a new filter to the query that will require that
// the ImageID property is equal to the passed value.
func (q *InstanceQuery) FindByImageID(v string) *InstanceQuery {
	return q.Where(kallax.Eq(Schema.Instance.ImageID, v))
}

// FindByContainerID adds a new filter to the query that will require that
// the ContainerID property is equal to the passed value.
func (q *InstanceQuery) FindByContainerID(v string) *InstanceQuery {
	return q.Where(kallax.Eq(Schema.Instance.ContainerID, v))
}

// FindByExpiresAt adds a new filter to the query that will require that
// the ExpiresAt property is equal to the passed value.
func (q *InstanceQuery) FindByExpiresAt(cond kallax.ScalarCond, v time.Time) *InstanceQuery {
	return q.Where(cond(Schema.Instance.ExpiresAt, v))
}

// FindByActive adds a new filter to the query that will require that
// the Active property is equal to the passed value.
func (q *InstanceQuery) FindByActive(v bool) *InstanceQuery {
	return q.Where(kallax.Eq(Schema.Instance.Active, v))
}

// FindByCleaned adds a new filter to the query that will require that
// the Cleaned property is equal to the passed value.
func (q *InstanceQuery) FindByCleaned(v bool) *InstanceQuery {
	return q.Where(kallax.Eq(Schema.Instance.Cleaned, v))
}

// InstanceResultSet is the set of results returned by a query to the
// database.
type InstanceResultSet struct {
	ResultSet kallax.ResultSet
	last      *Instance
	lastErr   error
}

// NewInstanceResultSet creates a new result set for rows of the type
// Instance.
func NewInstanceResultSet(rs kallax.ResultSet) *InstanceResultSet {
	return &InstanceResultSet{ResultSet: rs}
}

// Next fetches the next item in the result set and returns true if there is
// a next item.
// The result set is closed automatically when there are no more items.
func (rs *InstanceResultSet) Next() bool {
	if !rs.ResultSet.Next() {
		rs.lastErr = rs.ResultSet.Close()
		rs.last = nil
		return false
	}

	var record kallax.Record
	record, rs.lastErr = rs.ResultSet.Get(Schema.Instance.BaseSchema)
	if rs.lastErr != nil {
		rs.last = nil
	} else {
		var ok bool
		rs.last, ok = record.(*Instance)
		if !ok {
			rs.lastErr = fmt.Errorf("kallax: unable to convert record to *Instance")
			rs.last = nil
		}
	}

	return true
}

// Get retrieves the last fetched item from the result set and the last error.
func (rs *InstanceResultSet) Get() (*Instance, error) {
	return rs.last, rs.lastErr
}

// ForEach iterates over the complete result set passing every record found to
// the given callback. It is possible to stop the iteration by returning
// `kallax.ErrStop` in the callback.
// Result set is always closed at the end.
func (rs *InstanceResultSet) ForEach(fn func(*Instance) error) error {
	for rs.Next() {
		record, err := rs.Get()
		if err != nil {
			return err
		}

		if err := fn(record); err != nil {
			if err == kallax.ErrStop {
				return rs.Close()
			}

			return err
		}
	}
	return nil
}

// All returns all records on the result set and closes the result set.
func (rs *InstanceResultSet) All() ([]*Instance, error) {
	var result []*Instance
	for rs.Next() {
		record, err := rs.Get()
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

// One returns the first record on the result set and closes the result set.
func (rs *InstanceResultSet) One() (*Instance, error) {
	if !rs.Next() {
		return nil, kallax.ErrNotFound
	}

	record, err := rs.Get()
	if err != nil {
		return nil, err
	}

	if err := rs.Close(); err != nil {
		return nil, err
	}

	return record, nil
}

// Err returns the last error occurred.
func (rs *InstanceResultSet) Err() error {
	return rs.lastErr
}

// Close closes the result set.
func (rs *InstanceResultSet) Close() error {
	return rs.ResultSet.Close()
}

// NewSpec returns a new instance of Spec.
func NewSpec() (record *Spec) {
	return newSpec()
}

// GetID returns the primary key of the model.
func (r *Spec) GetID() kallax.Identifier {
	return (*kallax.ULID)(&r.ID)
}

// ColumnAddress returns the pointer to the value of the given column.
func (r *Spec) ColumnAddress(col string) (interface{}, error) {
	switch col {
	case "id":
		return (*kallax.ULID)(&r.ID), nil
	case "created_at":
		return &r.Timestamps.CreatedAt, nil
	case "updated_at":
		return &r.Timestamps.UpdatedAt, nil
	case "assignment_name":
		return &r.AssignmentName, nil
	case "data":
		return types.JSON(&r.Data), nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in Spec: %s", col)
	}
}

// Value returns the value of the given column.
func (r *Spec) Value(col string) (interface{}, error) {
	switch col {
	case "id":
		return r.ID, nil
	case "created_at":
		return r.Timestamps.CreatedAt, nil
	case "updated_at":
		return r.Timestamps.UpdatedAt, nil
	case "assignment_name":
		return r.AssignmentName, nil
	case "data":
		return types.JSON(r.Data), nil

	default:
		return nil, fmt.Errorf("kallax: invalid column in Spec: %s", col)
	}
}

// NewRelationshipRecord returns a new record for the relatiobship in the given
// field.
func (r *Spec) NewRelationshipRecord(field string) (kallax.Record, error) {
	switch field {
	case "Instances":
		return new(Instance), nil

	}
	return nil, fmt.Errorf("kallax: model Spec has no relationship %s", field)
}

// SetRelationship sets the given relationship in the given field.
func (r *Spec) SetRelationship(field string, rel interface{}) error {
	switch field {
	case "Instances":
		records, ok := rel.([]kallax.Record)
		if !ok {
			return fmt.Errorf("kallax: relationship field %s needs a collection of records, not %T", field, rel)
		}

		r.Instances = make([]*Instance, len(records))
		for i, record := range records {
			rel, ok := record.(*Instance)
			if !ok {
				return fmt.Errorf("kallax: element of type %T cannot be added to relationship %s", record, field)
			}
			r.Instances[i] = rel
		}
		return nil

	}
	return fmt.Errorf("kallax: model Spec has no relationship %s", field)
}

// SpecStore is the entity to access the records of the type Spec
// in the database.
type SpecStore struct {
	*kallax.Store
}

// NewSpecStore creates a new instance of SpecStore
// using a SQL database.
func NewSpecStore(db *sql.DB) *SpecStore {
	return &SpecStore{kallax.NewStore(db)}
}

// GenericStore returns the generic store of this store.
func (s *SpecStore) GenericStore() *kallax.Store {
	return s.Store
}

// SetGenericStore changes the generic store of this store.
func (s *SpecStore) SetGenericStore(store *kallax.Store) {
	s.Store = store
}

// Debug returns a new store that will print all SQL statements to stdout using
// the log.Printf function.
func (s *SpecStore) Debug() *SpecStore {
	return &SpecStore{s.Store.Debug()}
}

// DebugWith returns a new store that will print all SQL statements using the
// given logger function.
func (s *SpecStore) DebugWith(logger kallax.LoggerFunc) *SpecStore {
	return &SpecStore{s.Store.DebugWith(logger)}
}

func (s *SpecStore) relationshipRecords(record *Spec) []modelSaveFunc {
	var result []modelSaveFunc

	for i := range record.Instances {
		r := record.Instances[i]
		if !r.IsSaving() {
			r.AddVirtualColumn("spec_id", record.GetID())
			result = append(result, func(store *kallax.Store) error {
				_, err := (&InstanceStore{store}).Save(r)
				return err
			})
		}
	}

	return result
}

// Insert inserts a Spec in the database. A non-persisted object is
// required for this operation.
func (s *SpecStore) Insert(record *Spec) error {
	record.SetSaving(true)
	defer record.SetSaving(false)

	record.CreatedAt = record.CreatedAt.Truncate(time.Microsecond)
	record.UpdatedAt = record.UpdatedAt.Truncate(time.Microsecond)

	if err := record.BeforeSave(); err != nil {
		return err
	}

	records := s.relationshipRecords(record)

	if len(records) > 0 {
		return s.Store.Transaction(func(s *kallax.Store) error {
			if err := s.Insert(Schema.Spec.BaseSchema, record); err != nil {
				return err
			}

			for _, r := range records {
				if err := r(s); err != nil {
					return err
				}
			}

			return nil
		})
	}

	return s.Store.Insert(Schema.Spec.BaseSchema, record)
}

// Update updates the given record on the database. If the columns are given,
// only these columns will be updated. Otherwise all of them will be.
// Be very careful with this, as you will have a potentially different object
// in memory but not on the database.
// Only writable records can be updated. Writable objects are those that have
// been just inserted or retrieved using a query with no custom select fields.
func (s *SpecStore) Update(record *Spec, cols ...kallax.SchemaField) (updated int64, err error) {
	record.CreatedAt = record.CreatedAt.Truncate(time.Microsecond)
	record.UpdatedAt = record.UpdatedAt.Truncate(time.Microsecond)

	record.SetSaving(true)
	defer record.SetSaving(false)

	if err := record.BeforeSave(); err != nil {
		return 0, err
	}

	records := s.relationshipRecords(record)

	if len(records) > 0 {
		err = s.Store.Transaction(func(s *kallax.Store) error {
			updated, err = s.Update(Schema.Spec.BaseSchema, record, cols...)
			if err != nil {
				return err
			}

			for _, r := range records {
				if err := r(s); err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			return 0, err
		}

		return updated, nil
	}

	return s.Store.Update(Schema.Spec.BaseSchema, record, cols...)
}

// Save inserts the object if the record is not persisted, otherwise it updates
// it. Same rules of Update and Insert apply depending on the case.
func (s *SpecStore) Save(record *Spec) (updated bool, err error) {
	if !record.IsPersisted() {
		return false, s.Insert(record)
	}

	rowsUpdated, err := s.Update(record)
	if err != nil {
		return false, err
	}

	return rowsUpdated > 0, nil
}

// Delete removes the given record from the database.
func (s *SpecStore) Delete(record *Spec) error {
	return s.Store.Delete(Schema.Spec.BaseSchema, record)
}

// Find returns the set of results for the given query.
func (s *SpecStore) Find(q *SpecQuery) (*SpecResultSet, error) {
	rs, err := s.Store.Find(q)
	if err != nil {
		return nil, err
	}

	return NewSpecResultSet(rs), nil
}

// MustFind returns the set of results for the given query, but panics if there
// is any error.
func (s *SpecStore) MustFind(q *SpecQuery) *SpecResultSet {
	return NewSpecResultSet(s.Store.MustFind(q))
}

// Count returns the number of rows that would be retrieved with the given
// query.
func (s *SpecStore) Count(q *SpecQuery) (int64, error) {
	return s.Store.Count(q)
}

// MustCount returns the number of rows that would be retrieved with the given
// query, but panics if there is an error.
func (s *SpecStore) MustCount(q *SpecQuery) int64 {
	return s.Store.MustCount(q)
}

// FindOne returns the first row returned by the given query.
// `ErrNotFound` is returned if there are no results.
func (s *SpecStore) FindOne(q *SpecQuery) (*Spec, error) {
	q.Limit(1)
	q.Offset(0)
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	if !rs.Next() {
		return nil, kallax.ErrNotFound
	}

	record, err := rs.Get()
	if err != nil {
		return nil, err
	}

	if err := rs.Close(); err != nil {
		return nil, err
	}

	return record, nil
}

// FindAll returns a list of all the rows returned by the given query.
func (s *SpecStore) FindAll(q *SpecQuery) ([]*Spec, error) {
	rs, err := s.Find(q)
	if err != nil {
		return nil, err
	}

	return rs.All()
}

// MustFindOne returns the first row retrieved by the given query. It panics
// if there is an error or if there are no rows.
func (s *SpecStore) MustFindOne(q *SpecQuery) *Spec {
	record, err := s.FindOne(q)
	if err != nil {
		panic(err)
	}
	return record
}

// Reload refreshes the Spec with the data in the database and
// makes it writable.
func (s *SpecStore) Reload(record *Spec) error {
	return s.Store.Reload(Schema.Spec.BaseSchema, record)
}

// Transaction executes the given callback in a transaction and rollbacks if
// an error is returned.
// The transaction is only open in the store passed as a parameter to the
// callback.
func (s *SpecStore) Transaction(callback func(*SpecStore) error) error {
	if callback == nil {
		return kallax.ErrInvalidTxCallback
	}

	return s.Store.Transaction(func(store *kallax.Store) error {
		return callback(&SpecStore{store})
	})
}

// RemoveInstances removes the given items of the Instances field of the
// model. If no items are given, it removes all of them.
// The items will also be removed from the passed record inside this method.
// Note that is required that `Instances` is not empty. This method clears the
// the elements of Instances in a model, it does not retrieve them to know
// what relationships the model has.
func (s *SpecStore) RemoveInstances(record *Spec, deleted ...*Instance) error {
	var updated []*Instance
	var clear bool
	if len(deleted) == 0 {
		clear = true
		deleted = record.Instances
		if len(deleted) == 0 {
			return nil
		}
	}

	if len(deleted) > 1 {
		err := s.Store.Transaction(func(s *kallax.Store) error {
			for _, d := range deleted {
				var r kallax.Record = d

				if beforeDeleter, ok := r.(kallax.BeforeDeleter); ok {
					if err := beforeDeleter.BeforeDelete(); err != nil {
						return err
					}
				}

				if err := s.Delete(Schema.Instance.BaseSchema, d); err != nil {
					return err
				}

				if afterDeleter, ok := r.(kallax.AfterDeleter); ok {
					if err := afterDeleter.AfterDelete(); err != nil {
						return err
					}
				}
			}
			return nil
		})

		if err != nil {
			return err
		}

		if clear {
			record.Instances = nil
			return nil
		}
	} else {
		var r kallax.Record = deleted[0]
		if beforeDeleter, ok := r.(kallax.BeforeDeleter); ok {
			if err := beforeDeleter.BeforeDelete(); err != nil {
				return err
			}
		}

		var err error
		if afterDeleter, ok := r.(kallax.AfterDeleter); ok {
			err = s.Store.Transaction(func(s *kallax.Store) error {
				err := s.Delete(Schema.Instance.BaseSchema, r)
				if err != nil {
					return err
				}

				return afterDeleter.AfterDelete()
			})
		} else {
			err = s.Store.Delete(Schema.Instance.BaseSchema, deleted[0])
		}

		if err != nil {
			return err
		}
	}

	for _, r := range record.Instances {
		var found bool
		for _, d := range deleted {
			if d.GetID().Equals(r.GetID()) {
				found = true
				break
			}
		}
		if !found {
			updated = append(updated, r)
		}
	}
	record.Instances = updated
	return nil
}

// SpecQuery is the object used to create queries for the Spec
// entity.
type SpecQuery struct {
	*kallax.BaseQuery
}

// NewSpecQuery returns a new instance of SpecQuery.
func NewSpecQuery() *SpecQuery {
	return &SpecQuery{
		BaseQuery: kallax.NewBaseQuery(Schema.Spec.BaseSchema),
	}
}

// Select adds columns to select in the query.
func (q *SpecQuery) Select(columns ...kallax.SchemaField) *SpecQuery {
	if len(columns) == 0 {
		return q
	}
	q.BaseQuery.Select(columns...)
	return q
}

// SelectNot excludes columns from being selected in the query.
func (q *SpecQuery) SelectNot(columns ...kallax.SchemaField) *SpecQuery {
	q.BaseQuery.SelectNot(columns...)
	return q
}

// Copy returns a new identical copy of the query. Remember queries are mutable
// so make a copy any time you need to reuse them.
func (q *SpecQuery) Copy() *SpecQuery {
	return &SpecQuery{
		BaseQuery: q.BaseQuery.Copy(),
	}
}

// Order adds order clauses to the query for the given columns.
func (q *SpecQuery) Order(cols ...kallax.ColumnOrder) *SpecQuery {
	q.BaseQuery.Order(cols...)
	return q
}

// BatchSize sets the number of items to fetch per batch when there are 1:N
// relationships selected in the query.
func (q *SpecQuery) BatchSize(size uint64) *SpecQuery {
	q.BaseQuery.BatchSize(size)
	return q
}

// Limit sets the max number of items to retrieve.
func (q *SpecQuery) Limit(n uint64) *SpecQuery {
	q.BaseQuery.Limit(n)
	return q
}

// Offset sets the number of items to skip from the result set of items.
func (q *SpecQuery) Offset(n uint64) *SpecQuery {
	q.BaseQuery.Offset(n)
	return q
}

// Where adds a condition to the query. All conditions added are concatenated
// using a logical AND.
func (q *SpecQuery) Where(cond kallax.Condition) *SpecQuery {
	q.BaseQuery.Where(cond)
	return q
}

func (q *SpecQuery) WithInstances(cond kallax.Condition) *SpecQuery {
	q.AddRelation(Schema.Instance.BaseSchema, "Instances", kallax.OneToMany, cond)
	return q
}

// FindByID adds a new filter to the query that will require that
// the ID property is equal to one of the passed values; if no passed values,
// it will do nothing.
func (q *SpecQuery) FindByID(v ...kallax.ULID) *SpecQuery {
	if len(v) == 0 {
		return q
	}
	values := make([]interface{}, len(v))
	for i, val := range v {
		values[i] = val
	}
	return q.Where(kallax.In(Schema.Spec.ID, values...))
}

// FindByCreatedAt adds a new filter to the query that will require that
// the CreatedAt property is equal to the passed value.
func (q *SpecQuery) FindByCreatedAt(cond kallax.ScalarCond, v time.Time) *SpecQuery {
	return q.Where(cond(Schema.Spec.CreatedAt, v))
}

// FindByUpdatedAt adds a new filter to the query that will require that
// the UpdatedAt property is equal to the passed value.
func (q *SpecQuery) FindByUpdatedAt(cond kallax.ScalarCond, v time.Time) *SpecQuery {
	return q.Where(cond(Schema.Spec.UpdatedAt, v))
}

// FindByAssignmentName adds a new filter to the query that will require that
// the AssignmentName property is equal to the passed value.
func (q *SpecQuery) FindByAssignmentName(v string) *SpecQuery {
	return q.Where(kallax.Eq(Schema.Spec.AssignmentName, v))
}

// SpecResultSet is the set of results returned by a query to the
// database.
type SpecResultSet struct {
	ResultSet kallax.ResultSet
	last      *Spec
	lastErr   error
}

// NewSpecResultSet creates a new result set for rows of the type
// Spec.
func NewSpecResultSet(rs kallax.ResultSet) *SpecResultSet {
	return &SpecResultSet{ResultSet: rs}
}

// Next fetches the next item in the result set and returns true if there is
// a next item.
// The result set is closed automatically when there are no more items.
func (rs *SpecResultSet) Next() bool {
	if !rs.ResultSet.Next() {
		rs.lastErr = rs.ResultSet.Close()
		rs.last = nil
		return false
	}

	var record kallax.Record
	record, rs.lastErr = rs.ResultSet.Get(Schema.Spec.BaseSchema)
	if rs.lastErr != nil {
		rs.last = nil
	} else {
		var ok bool
		rs.last, ok = record.(*Spec)
		if !ok {
			rs.lastErr = fmt.Errorf("kallax: unable to convert record to *Spec")
			rs.last = nil
		}
	}

	return true
}

// Get retrieves the last fetched item from the result set and the last error.
func (rs *SpecResultSet) Get() (*Spec, error) {
	return rs.last, rs.lastErr
}

// ForEach iterates over the complete result set passing every record found to
// the given callback. It is possible to stop the iteration by returning
// `kallax.ErrStop` in the callback.
// Result set is always closed at the end.
func (rs *SpecResultSet) ForEach(fn func(*Spec) error) error {
	for rs.Next() {
		record, err := rs.Get()
		if err != nil {
			return err
		}

		if err := fn(record); err != nil {
			if err == kallax.ErrStop {
				return rs.Close()
			}

			return err
		}
	}
	return nil
}

// All returns all records on the result set and closes the result set.
func (rs *SpecResultSet) All() ([]*Spec, error) {
	var result []*Spec
	for rs.Next() {
		record, err := rs.Get()
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

// One returns the first record on the result set and closes the result set.
func (rs *SpecResultSet) One() (*Spec, error) {
	if !rs.Next() {
		return nil, kallax.ErrNotFound
	}

	record, err := rs.Get()
	if err != nil {
		return nil, err
	}

	if err := rs.Close(); err != nil {
		return nil, err
	}

	return record, nil
}

// Err returns the last error occurred.
func (rs *SpecResultSet) Err() error {
	return rs.lastErr
}

// Close closes the result set.
func (rs *SpecResultSet) Close() error {
	return rs.ResultSet.Close()
}

type schema struct {
	Instance *schemaInstance
	Spec     *schemaSpec
}

type schemaInstance struct {
	*kallax.BaseSchema
	ID          kallax.SchemaField
	CreatedAt   kallax.SchemaField
	UpdatedAt   kallax.SchemaField
	SpecFK      kallax.SchemaField
	ImageID     kallax.SchemaField
	ContainerID kallax.SchemaField
	ExpiresAt   kallax.SchemaField
	Active      kallax.SchemaField
	Cleaned     kallax.SchemaField
}

type schemaSpec struct {
	*kallax.BaseSchema
	ID             kallax.SchemaField
	CreatedAt      kallax.SchemaField
	UpdatedAt      kallax.SchemaField
	AssignmentName kallax.SchemaField
	Data           kallax.SchemaField
}

var Schema = &schema{
	Instance: &schemaInstance{
		BaseSchema: kallax.NewBaseSchema(
			"instances",
			"__instance",
			kallax.NewSchemaField("id"),
			kallax.ForeignKeys{
				"Spec": kallax.NewForeignKey("spec_id", true),
			},
			func() kallax.Record {
				return new(Instance)
			},
			false,
			kallax.NewSchemaField("id"),
			kallax.NewSchemaField("created_at"),
			kallax.NewSchemaField("updated_at"),
			kallax.NewSchemaField("spec_id"),
			kallax.NewSchemaField("image_id"),
			kallax.NewSchemaField("container_id"),
			kallax.NewSchemaField("expires_at"),
			kallax.NewSchemaField("active"),
			kallax.NewSchemaField("cleaned"),
		),
		ID:          kallax.NewSchemaField("id"),
		CreatedAt:   kallax.NewSchemaField("created_at"),
		UpdatedAt:   kallax.NewSchemaField("updated_at"),
		SpecFK:      kallax.NewSchemaField("spec_id"),
		ImageID:     kallax.NewSchemaField("image_id"),
		ContainerID: kallax.NewSchemaField("container_id"),
		ExpiresAt:   kallax.NewSchemaField("expires_at"),
		Active:      kallax.NewSchemaField("active"),
		Cleaned:     kallax.NewSchemaField("cleaned"),
	},
	Spec: &schemaSpec{
		BaseSchema: kallax.NewBaseSchema(
			"specs",
			"__spec",
			kallax.NewSchemaField("id"),
			kallax.ForeignKeys{
				"Instances": kallax.NewForeignKey("spec_id", false),
			},
			func() kallax.Record {
				return new(Spec)
			},
			false,
			kallax.NewSchemaField("id"),
			kallax.NewSchemaField("created_at"),
			kallax.NewSchemaField("updated_at"),
			kallax.NewSchemaField("assignment_name"),
			kallax.NewSchemaField("data"),
		),
		ID:             kallax.NewSchemaField("id"),
		CreatedAt:      kallax.NewSchemaField("created_at"),
		UpdatedAt:      kallax.NewSchemaField("updated_at"),
		AssignmentName: kallax.NewSchemaField("assignment_name"),
		Data:           kallax.NewSchemaField("data"),
	},
}
