package soft_delete

import (
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type DeletedAt uint

var (
	FlagDeleted = 1
	FlagActived = 0
)

func (DeletedAt) QueryClauses(f *schema.Field) []clause.Interface {
	return []clause.Interface{SoftDeleteQueryClause{Field: f}}
}

type SoftDeleteQueryClause struct {
	Field *schema.Field
}

func (sd SoftDeleteQueryClause) Name() string {
	return ""
}

func (sd SoftDeleteQueryClause) Build(clause.Builder) {
}

func (sd SoftDeleteQueryClause) MergeClause(*clause.Clause) {
}

func (sd SoftDeleteQueryClause) ModifyStatement(stmt *gorm.Statement) {
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

		if sd.Field.DefaultValue == "null" {
			stmt.AddClause(clause.Where{Exprs: []clause.Expression{
				clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: sd.Field.DBName}, Value: nil},
			}})
		} else {
			stmt.AddClause(clause.Where{Exprs: []clause.Expression{
				clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: sd.Field.DBName}, Value: FlagActived},
			}})
		}
		stmt.Clauses["soft_delete_enabled"] = clause.Clause{}
	}
}

func (DeletedAt) DeleteClauses(f *schema.Field) []clause.Interface {
	settings := schema.ParseTagSetting(f.TagSettings["SOFTDELETE"], ",")
	softDeleteClause := SoftDeleteDeleteClause{
		Field:    f,
		Flag:     settings["FLAG"] != "",
		TimeType: getTimeType(settings),
	}
	if v := settings["DELETEDATFIELD"]; v != "" { // DeletedAtField
		softDeleteClause.DeleteAtField = f.Schema.LookUpField(v)
	}
	return []clause.Interface{softDeleteClause}
}

func (DeletedAt) UpdateClauses(f *schema.Field) []clause.Interface {
	return []clause.Interface{SoftDeleteUpdateClause{Field: f}}
}

type SoftDeleteUpdateClause struct {
	Field *schema.Field
}

func (sd SoftDeleteUpdateClause) Name() string {
	return ""
}

func (sd SoftDeleteUpdateClause) Build(clause.Builder) {
}

func (sd SoftDeleteUpdateClause) MergeClause(*clause.Clause) {
}

func (sd SoftDeleteUpdateClause) ModifyStatement(stmt *gorm.Statement) {
	if stmt.SQL.Len() == 0 && !stmt.Statement.Unscoped {
		SoftDeleteQueryClause(sd).ModifyStatement(stmt)
	}
}

type SoftDeleteDeleteClause struct {
	Field         *schema.Field
	Flag          bool
	TimeType      schema.TimeType
	DeleteAtField *schema.Field
}

func (sd SoftDeleteDeleteClause) Name() string {
	return ""
}

func (sd SoftDeleteDeleteClause) Build(clause.Builder) {
}

func (sd SoftDeleteDeleteClause) MergeClause(*clause.Clause) {
}

func (sd SoftDeleteDeleteClause) ModifyStatement(stmt *gorm.Statement) {
	if stmt.SQL.Len() == 0 && !stmt.Statement.Unscoped {
		var (
			curTime = stmt.DB.NowFunc()
			set     clause.Set
		)

		if deleteAtField := sd.DeleteAtField; deleteAtField != nil {
			var value interface{}
			if deleteAtField.GORMDataType == "time" {
				value = curTime
			} else {
				value = sd.timeToUnix(curTime)
			}
			set = append(set, clause.Assignment{Column: clause.Column{Name: deleteAtField.DBName}, Value: value})
			stmt.SetColumn(deleteAtField.DBName, value, true)
		}

		if sd.Flag {
			set = append(clause.Set{{Column: clause.Column{Name: sd.Field.DBName}, Value: FlagDeleted}}, set...)
			stmt.SetColumn(sd.Field.DBName, FlagDeleted, true)
			stmt.AddClause(set)
		} else {
			var curUnix = sd.timeToUnix(curTime)
			set = append(clause.Set{{Column: clause.Column{Name: sd.Field.DBName}, Value: curUnix}}, set...)
			stmt.AddClause(set)
			stmt.SetColumn(sd.Field.DBName, curUnix, true)
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

		SoftDeleteQueryClause{Field: sd.Field}.ModifyStatement(stmt)
		stmt.AddClauseIfNotExists(clause.Update{})
		stmt.Build(stmt.DB.Callback().Update().Clauses...)
	}
}

func (sd SoftDeleteDeleteClause) timeToUnix(curTime time.Time) int64 {
	switch sd.TimeType {
	case schema.UnixNanosecond:
		return curTime.UnixNano()
	case schema.UnixMillisecond:
		return curTime.UnixNano() / 1e6
	default:
		return curTime.Unix()
	}
}

func getTimeType(settings map[string]string) schema.TimeType {
	if settings["NANO"] != "" {
		return schema.UnixNanosecond
	}

	if settings["MILLI"] != "" {
		return schema.UnixMillisecond
	}

	fieldUnit := strings.ToUpper(settings["DELETEDATFIELDUNIT"])

	if fieldUnit == "NANO" {
		return schema.UnixNanosecond
	}

	if fieldUnit == "MILLI" {
		return schema.UnixMillisecond
	}

	return schema.UnixSecond
}
