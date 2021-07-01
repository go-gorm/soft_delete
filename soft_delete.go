package soft_delete

import (
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type DeletedAt uint

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
	if _, ok := stmt.Clauses["soft_delete_enabled"]; !ok {
		if c, ok := stmt.Clauses["WHERE"]; ok {
			if where, ok := c.Expression.(clause.Where); ok && len(where.Exprs) > 1 {
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
				clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: sd.Field.DBName}, Value: 0},
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
		TimeType: schema.UnixSecond,
	}

	// flag is much more priority
	if !softDeleteClause.Flag {
		if settings["NANO"] != "" {
			softDeleteClause.TimeType = schema.UnixNanosecond
		} else if settings["MILLI"] != "" {
			softDeleteClause.TimeType = schema.UnixMillisecond
		} else {
			softDeleteClause.TimeType = schema.UnixSecond
		}
	}

	if v := settings["DELETEDATFIELD"]; v != "" { // DeletedAtField
		softDeleteClause.DeleteAtField = f.Schema.LookUpField(v)
	}

	return []clause.Interface{softDeleteClause}
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
	if stmt.SQL.String() == "" {
		curTime := stmt.DB.NowFunc()
		if sd.Flag {
			set := clause.Set{{Column: clause.Column{Name: sd.Field.DBName}, Value: 1}}
			stmt.SetColumn(sd.Field.DBName, 1, true)

			if sd.DeleteAtField != nil {
				set = append(set, clause.Assignment{Column: clause.Column{Name: sd.DeleteAtField.DBName}, Value: curTime.Unix()})
				stmt.SetColumn(sd.DeleteAtField.DBName, curTime, true)
			}

			stmt.AddClause(set)
		} else {
			var curUnix int64 = 0
			if sd.TimeType == schema.UnixNanosecond {
				curUnix = curTime.UnixNano()
			} else if sd.TimeType == schema.UnixMillisecond {
				curUnix = curTime.UnixNano() / 1e6
			} else {
				curUnix = curTime.Unix()
			}
			stmt.AddClause(clause.Set{{Column: clause.Column{Name: sd.Field.DBName}, Value: curUnix}})
			stmt.SetColumn(sd.Field.DBName, curUnix, true)
		}

		if stmt.Schema != nil {
			_, queryValues := schema.GetIdentityFieldValuesMap(stmt.ReflectValue, stmt.Schema.PrimaryFields)
			column, values := schema.ToQueryValues(stmt.Table, stmt.Schema.PrimaryFieldDBNames, queryValues)

			if len(values) > 0 {
				stmt.AddClause(clause.Where{Exprs: []clause.Expression{clause.IN{Column: column, Values: values}}})
			}

			if stmt.ReflectValue.CanAddr() && stmt.Dest != stmt.Model && stmt.Model != nil {
				_, queryValues = schema.GetIdentityFieldValuesMap(reflect.ValueOf(stmt.Model), stmt.Schema.PrimaryFields)
				column, values = schema.ToQueryValues(stmt.Table, stmt.Schema.PrimaryFieldDBNames, queryValues)

				if len(values) > 0 {
					stmt.AddClause(clause.Where{Exprs: []clause.Expression{clause.IN{Column: column, Values: values}}})
				}
			}
		}

		if _, ok := stmt.Clauses["WHERE"]; !stmt.DB.AllowGlobalUpdate && !ok {
			stmt.DB.AddError(gorm.ErrMissingWhereClause)
		} else {
			SoftDeleteQueryClause{Field: sd.Field}.ModifyStatement(stmt)
		}

		stmt.AddClauseIfNotExists(clause.Update{})
		stmt.Build("UPDATE", "SET", "WHERE")
	}
}
