## DAG Generator

`astro dag-generate` uses [go.rice](https://github.com/GeertJohan/go.rice) to package txt code snippets into the executable

```
rice embed-go
go build
```

Code snippets can be located inside the `daggenerate` top level directory.

---

TODO:

* Generate real code
* More sources + destinations
* Generate help text and validation dynamically by reading the files
* Snippets could live in Astronomer Registry, requiring auth + internet connection to generate DAGs
