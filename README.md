# Pgx Batch
This library offers a way to run multiple batched queries without relinquishing the `pgxtype.Querier` interface.
Whereas `pgxtype.Querier` immediately returns, the `pgx.Batch` type does not return responses immediately. This makes it hard to make reusable methods that load data, and combine them in 1 batch.
This library offers the best of both worlds.

Suppose you want to insert data into 3 different tables: people, groups and pets. Please find sample data in `example.sql`.

```sql
INSERT INTO people (firstname, lastname) VALUES ('Joe', 'Cool') RETURNING id;
INSERT INTO groups (name) VALUES ('Cat lovers') RETURNING id;
INSERT INTO pets (name, species) VALUES ('Tom', 'Cat') RETURNING id;
```

## Naive implementation
If those queries are written like below, the inserts happen one by one, each taking one roundtrip.

```go
var q *pgx.Pool
var row pgx.Row

row = q.QueryRow(ctx, "INSERT INTO people (firstname, lastname) VALUES ('Joe', 'Cool') RETURNING id")
var id int
if err = row.Scan(&id); err == nil {
    log.Println("Inserted person with ID", id)
}

row = q.QueryRow(ctx, "INSERT INTO groups (name) VALUES ('Cat lovers') RETURNING id")
var id int
if err = row.Scan(&id); err == nil {
    log.Println("Inserted group with ID", id)
}

row = q.QueryRow(ctx, "INSERT INTO pets (name, species) VALUES ('Tom', 'Cat') RETURNING id")
var id int
if err = row.Scan(&id); err == nil {
    log.Println("Inserted pets with ID", id)
}
```

## pgx.Batch implementation
While this is fast if the database is near, it is not if the database is globally distributed.
With pgx.Batch this can be rewritten to below code. This is not easily separable into a repository pattern.

```go
b := &pgx.Batch{}
b.Queue("INSERT INTO people (firstname, lastname) VALUES ('Joe', 'Cool') RETURNING id")
b.Queue("INSERT INTO groups (name) VALUES ('Cat lovers') RETURNING id")
b.Queue("INSERT INTO pets (name, species) VALUES ('Tom', 'Cat') RETURNING id")

var q *pgx.Pool
results := q.SendBatch(context.Background(), b)

var err error
var ids []int = []int{0,0,0}
err = results.QueryRow().Scan(&ids[0])
err = results.QueryRow().Scan(&ids[1])
err = results.QueryRow().Scan(&ids[2])
```

With pgxbatch, this can be rewritten to:

```go
b := &pgxbatch.Batch{}

b.RegisterActor(func(q pgxtype.Querier) (err error) {
    row := q.QueryRow(ctx, "INSERT INTO people (firstname, lastname) VALUES ('Joe', 'Cool') RETURNING id")
    var id int
    if err = row.Scan(&id); err == nil {
        log.Println("Inserted person with ID", id)
    }
    return
})
b.RegisterActor(func(q pgxtype.Querier) (err error) {
    row := q.QueryRow(ctx, "INSERT INTO groups (name) VALUES ('Cat lovers') RETURNING id")
    var id int
    if err = row.Scan(&id); err == nil {
        log.Println("Inserted group with ID", id)
    }
    return
})
b.RegisterActor(func(q pgxtype.Querier) (err error) {
    row := q.QueryRow(ctx, "INSERT INTO pets (name, species) VALUES ('Tom', 'Cat') RETURNING id")
    var id int
    if err = row.Scan(&id); err == nil {
        log.Println("Inserted pets with ID", id)
    }
    return
})

err := b.Run()
```

This might not seem a big change, but with ORM libraries or helper functions I think this actor batch pattern scales better.
