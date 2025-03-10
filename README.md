# Soft Delete

[https://gorm.io/docs/delete.html#Soft-Delete](https://gorm.io/docs/delete.html#Soft-Delete)

```go
import "gorm.io/plugin/soft_delete"

type User struct {
  ID        uint
  Name      string
  DeletedAt soft_delete.DeletedAt
}

// Query
SELECT * FROM users WHERE deleted_at = 0;

// Delete
UPDATE users SET deleted_at = /* current unix second */ WHERE ID = 1;
```

### Specify Time Unit

We now support ms and ns timestamp when filling the `deleted_at` col, only need to specify tag `gorm:"softDelete:milli"` or `gorm:"softDelete:nano"`.

```go
type User struct {
  ID    uint
  Name  string
  DeletedAt soft_delete.DeletedAt `gorm:"softDelete:milli"`
  // DeletedAt soft_delete.DeletedAt `gorm:"softDelete:nano"`
}

// Query
SELECT * FROM users WHERE deleted_at = 0;

// Delete
UPDATE users SET deleted_at = /* current unix milli second or nano second */ WHERE ID = 1;
```

## Flag Mode

flag mode will use `0`, `1` to mark data as deleted or not, `1` means deleted

```go
type User struct {
  ID    uint
  Name  string
  IsDel soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

// Query
SELECT * FROM users WHERE is_del = 0;

// Delete
UPDATE users SET is_del = 1 WHERE ID = 1;
```


## Mixed Mode

mixed mode will use `0`, `1` to mark data as deleted or not, `1` means deleted, and store delete time

```go
type User struct {
  ID        uint
  Name      string
  DeletedAt time.Time
  IsDel     soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt"`
}

// Query
SELECT * FROM users WHERE is_del = 0;

// Delete
UPDATE users SET is_del = 1, deleted_at = /* current unix second */ WHERE ID = 1;
```

support mixed mode specify time unit, e.g. `gorm:"softDelete:flag,DeletedAtField:DeletedAt"` or `gorm:"softDelete:flag,DeletedAtField:DeletedAt,DeletedAtFieldUnit:milli"` or `gorm:"softDelete:flag,DeletedAtField:DeletedAt,DeletedAtFieldUnit:nano"`.

```go
type User struct {
  ID        uint
  Name      string
  DeletedAt int64
  IsDel     soft_delete.DeletedAt `gorm:"softDelete:flag,DeletedAtField:DeletedAt,DeletedAtFieldUnit:milli"`
}

// Query
SELECT * FROM users WHERE is_del = 0;

// Delete
UPDATE users SET is_del = 1, deleted_at = /* current unix milli second second*/ WHERE ID = 1;
```

## Mixed Mode with Delete ID Field
#### Maintaining Unique Key Integrity
This allows you to record the original ID of a deleted record in another field. By doing so, you can maintain the integrity of unique keys by allowing new records with the same unique key to be inserted without conflict.
#### Example
Assume we have a User model where the Email field needs to be unique. By storing the original ID in the DeletedId field and creating a composite unique key with Email and DeletedId, you can insert a new record without violating the unique constraint even after soft deleting an existing record.
```go
type User struct {
ID            uint
Name          string
Email         string
DeletedId     uint // Stores the original ID of the deleted record
IsDel         soft_delete.DeletedAt    `gorm:"softDelete:flag,DeletedIDField:DeletedId,DeletedIDFromField:ID"` // use `1` `0`
}

// Query
SELECT * FROM users WHERE is_del = 0;

// Delete
UPDATE users SET is_del = 1, deleted_id = /* value from ID */ WHERE ID = 1;
```
