# uAssign

uAssign is a system to manage *NIX terminal assignments. It uses Docker
containers to build real, interactive environments to an instructor's
specification, proxying a terminal over WebSockets to a user. The full
lifecycle of containers is managed, including the ability for users to
disconnect and reconnect with their state preserved.

More information about this project can be found in my
[master's thesis](https://www.ideals.illinois.edu/handle/2142/101068),
as well as in the `docs` directory.


## Developer notes

This repo has switched to Go modules to make this project more accessible,
as it can now be cloned and built outside of `$GOPATH`. For now, dependencies
are still vendored to make mounting the project into a Docker container
(see `docker-compose.yml`) work without resolving and downloading dependencies
at startup or mounting the host's mod cache directly. `docker-compose.yml`
runs assuming `vendor/` is correctly populated.

Use the `gomod.sh` script to properly update `go.mod`. This project uses
Docker libraries pretty heavily, the most important of which are not yet
compatible with Go modules. `gomod.sh` will update the more troubled
Docker libraries to `master`, replace `go.uuid` with the gofrs fork,
update the rest of the libraries normally, then populate `vendor/`. It
should be equivalent to using `dep` with some dependencies constrained
to `master`.

TODOs relating to Go modules:

- Switch from the unmaintained `mattes/migrate` to the maintained
    `golang-migrate/migrate`; was blocked by needing modules for versioned
    import paths.
- Run all `go generate` tooling with `gobin`/`tools.go` for versioning.
