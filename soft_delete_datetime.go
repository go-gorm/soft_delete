package soft_delete

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type DeletedDateTime sql.NullTime

var (
	//DateTimeZero FlagActived
	DateTimeZero  time.Time
	DefaultLayout = "2006-01-02 15:04:05"
)

func init() {

	DateTimeZero, _ = time.Parse(DefaultLayout, "1970-01-01 00:00:01")
}

// Scan implements the Scanner interface.
func (n *DeletedDateTime) Scan(value interface{}) error {
	return (*sql.NullTime)(n).Scan(value)
}

// Value implements the driver Valuer interface.
func (n DeletedDateTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time, nil
}

func (DeletedDateTime) QueryClauses(f *schema.Field) []clause.Interface {
	return []clause.Interface{SoftDeleteDateTimeQueryClause{Field: f}}
}

type SoftDeleteDateTimeQueryClause struct {
	Field *schema.Field
}

func (sd SoftDeleteDateTimeQueryClause) Name() string {
	return ""
}

func (sd SoftDeleteDateTimeQueryClause) Build(clause.Builder) {
}

func (sd SoftDeleteDateTimeQueryClause) MergeClause(*clause.Clause) {
}

func (sd SoftDeleteDateTimeQueryClause) ModifyStatement(stmt *gorm.Statement) {
	if _, ok := stmt.Clauses["soft_delete_enabled"]; !ok && !stmt.Statement.Unscoped {
		if c, ok := stmt.Clauses["WHERE"]; ok {
			if where, ok := c.Expression.(clause.Where); ok && len(where.Exprs) >= 1 {
				for _, expr := range where.Exprs {
					if orCond, ok := expr.(clause.OrConditions); ok && len(orCond.Exprs) == 1 {
						where.Exprs = []clause.Expression{clause.And(where.Exprs...)}
						c.Expression = where
						stmt.Clauses["WHERE"] = c
						break
					}
				}
			}
		}

		stmt.AddClause(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: sd.Field.DBName}, Value: DateTimeZero.Format(DefaultLayout)},
		}})
		stmt.Clauses["soft_delete_enabled"] = clause.Clause{}
	}
}

func (DeletedDateTime) UpdateClauses(f *schema.Field) []clause.Interface {
	return []clause.Interface{SoftDeleteDateTimeUpdateClause{Field: f}}
}

type SoftDeleteDateTimeUpdateClause struct {
	Field *schema.Field
}

func (sd SoftDeleteDateTimeUpdateClause) Name() string {
	return ""
}

func (sd SoftDeleteDateTimeUpdateClause) Build(clause.Builder) {
}

func (sd SoftDeleteDateTimeUpdateClause) MergeClause(*clause.Clause) {
}

func (sd SoftDeleteDateTimeUpdateClause) ModifyStatement(stmt *gorm.Statement) {
	if stmt.SQL.Len() == 0 && !stmt.Statement.Unscoped {
		SoftDeleteDateTimeQueryClause(sd).ModifyStatement(stmt)
	}
}

func (DeletedDateTime) DeleteClauses(f *schema.Field) []clause.Interface {
	settings := schema.ParseTagSetting(f.TagSettings["SOFTDELETE"], ",")
	softDeleteClause := SoftDeleteDateTimeDeleteClause{
		Field:    f,
		Flag:     settings["FLAG"] != "",
		TimeType: getTimeType(settings),
	}
	if v := settings["DELETEDATETIMEFIELD"]; v != "" { // DeleteDateTimeField
		softDeleteClause.DeleteDateTimeField = f.Schema.LookUpField(v)
	}
	return []clause.Interface{softDeleteClause}
}

type SoftDeleteDateTimeDeleteClause struct {
	Field               *schema.Field
	Flag                bool
	TimeType            schema.TimeType
	DeleteDateTimeField *schema.Field
}

func (sd SoftDeleteDateTimeDeleteClause) Name() string {
	return ""
}

func (sd SoftDeleteDateTimeDeleteClause) Build(clause.Builder) {
}

func (sd SoftDeleteDateTimeDeleteClause) MergeClause(*clause.Clause) {
}

func (sd SoftDeleteDateTimeDeleteClause) ModifyStatement(stmt *gorm.Statement) {
	if stmt.SQL.Len() == 0 && !stmt.Statement.Unscoped {
		var (
			curTime = stmt.DB.NowFunc()
			set     clause.Set
		)

		if deleteAtField := sd.DeleteDateTimeField; deleteAtField != nil {
			var value interface{}
			set = append(set, clause.Assignment{Column: clause.Column{Name: deleteAtField.DBName}, Value: curTime})
			stmt.SetColumn(deleteAtField.DBName, value, true)
		}

		if sd.Flag {
			set = append(clause.Set{{Column: clause.Column{Name: sd.Field.DBName}, Value: FlagDeleted}}, set...)
			stmt.SetColumn(sd.Field.DBName, FlagDeleted, true)
			stmt.AddClause(set)
		} else {
			set = append(clause.Set{{Column: clause.Column{Name: sd.Field.DBName}, Value: curTime}}, set...)
			stmt.AddClause(set)
			stmt.SetColumn(sd.Field.DBName, curTime, true)
		}

		if stmt.Schema != nil {
			_, queryValues := schema.GetIdentityFieldValuesMap(stmt.Context, stmt.ReflectValue, stmt.Schema.PrimaryFields)
			column, values := schema.ToQueryValues(stmt.Table, stmt.Schema.PrimaryFieldDBNames, queryValues)

			if len(values) > 0 {
				stmt.AddClause(clause.Where{Exprs: []clause.Expression{clause.IN{Column: column, Values: values}}})
			}

			if stmt.ReflectValue.CanAddr() && stmt.Dest != stmt.Model && stmt.Model != nil {
				_, queryValues = schema.GetIdentityFieldValuesMap(stmt.Context, reflect.ValueOf(stmt.Model), stmt.Schema.PrimaryFields)
				column, values = schema.ToQueryValues(stmt.Table, stmt.Schema.PrimaryFieldDBNames, queryValues)

				if len(values) > 0 {
					stmt.AddClause(clause.Where{Exprs: []clause.Expression{clause.IN{Column: column, Values: values}}})
				}
			}
		}

		SoftDeleteDateTimeQueryClause{Field: sd.Field}.ModifyStatement(stmt)
		stmt.AddClauseIfNotExists(clause.Update{})
		stmt.Build(stmt.DB.Callback().Update().Clauses...)
	}
}
