# Assignments

## Directory structure

Assignments are held in a single directory. They are referred to by name.
Assignments can be nested, like:

```
assignments
├── 00_basic_use
│   └── Dockerfile.tmpl
├── archive
│   ├── tar_compress
│   │   ├── context
│   │   │   ├── grade
│   │   │   │   └── grade.go
│   │   │   └── setup
│   │   │       └── setup.go
│   │   ├── Dockerfile.tmpl
│   │   └── index.js
│   └── tar_extract
│       ├── context
│       │   └── setup
│       │       └── setup.go
│       ├── Dockerfile.tmpl
│       └── index.js
├── compile
│   ├── c
│   │   ├── context
│   │   │   └── main.c
│   │   ├── Dockerfile.tmpl
│   │   └── index.js
│   └── c_o
│       ├── context
│       │   ├── grade
│       │   │   └── grade.go
│       │   └── main.c
│       ├── Dockerfile.tmpl
│       └── index.js
...
```

For example, to refer to the `tar_extract` assignment, you can refer to it as
`archive.tar_extract` when creating a spec.

An assignment directory consists of:

-   `index.js`, a CommonJS-style module. This module must export a `generate`
    function, which accepts a JavaScript object with the spec data, (that comes from
    the `data` object from the PrairieLearn question), and returns
    another JavaScript object with the generated instance information.
-   `context`, an optional directory which is the build context given to the
    docker daemon. This directory behaves like a regular docker context,
    respecting `.dockerignore`, with the difference that it doesn't contain a
    Dockerfile. If this directory doesn't exist, then an empty context will be
    used during build.


## Creating an assignment

Create a directory for the assignment. Then, create an `index.js` file in that
assignment directory. An example of an `index.js` file:

```javascript
var _ = require('lodash');

exports.generate = function(data) {
    var secret = data.secret;
    var filename = data.filename + ".c";
    var cSource = readFile("./context/main.c");

    return {
        imageName: "jakebailey/ua-cs126-docker:clang",
        init: true,
        postBuild: [
            {
                action: "append",
                user: "root",
                filename: "/usr/include/stdio.h",
                contents: "char _[] = { "
            },
            {
                action: "exec",
                user: "root",
                cmd: ["sh", "-c", "printf '" + secret
                    + "' | od -A n -t d1 | "
                    + "xargs -n1 printf '%d, ' >> /usr/include/stdio.h"],
            },
            {
                action: "append",
                user: "root",
                filename: "/usr/include/stdio.h",
                contents: "0 };\n"
            },
            {
                action: "write",
                user: "student",
                filename: "/home/student/" + filename,
                contents: cSource
            }
        ],
        user: "student",
        cmd: ["bash"],
        env: ["FOO=BAR"],
        workingDir: "/home/student"
    }
};
```

This example shows a number of capabilities of the assignments. Listed, they
are:

-   Modules can be included with `require()`, as in NodeJS, allowing the use of
    common JS libraries, or including modules in other files (for now, only
    within the assignment directory).
-   The `readFile` function reads files within the assignment directory,
    as strings.
-   `btoa` and `atob` allow for base64 encoding and decoding, respectively,
    as in browsers.

Most importantly, the `generate` function returns a JS object which describes
the instance, including the base docker image needed, what to do after the
image is built, as well as the command to expose to the user over the eventual
websocket.

-   `imageName` is the name of an image to begin with. In this example, this is
    an Alpine Linux image with some sane defaults as well as the clang
    compiler. If this image isn't present on the server, it will be pulled the
    first time it is needed. Alternatively, the `generate` function can
    set `dockerfile` instead of `imageName`, which is a string which contains
    a Dockerfile that can be used to build a custom image (rather than using
    something prebuilt).
-   `init` enables running an init system in the docker container, which can
    manage zombies. The default is currently `false`, but may be changed in the
    future.
-   `postBuild` is a list of "actions", which will be run on the container
    after it is built (before the user ever gets access). This offers a more
    performant alternative to using the docker build system.
-   `user`, `cmd`, `env`, and `workingDir` set the environment that the user
    will have access to. Generally, this is some non-root account, in a shell,
    in some directory (likely home).

The `postBuild` attribute is a list of "actions", which perform various tasks
on the container. All actions accept both `user` and `workingDir`, to manage
which user the container will think is doing the action.  Currently, three
actions are supported:

-   `exec` runs a command on the container. `cmd` sets the command, argv style.
    `env` sets environment variables. `stdin` is an optional string which will
    be fed into the command over stdin. The server will run the command, and
    will error out if the command fails (non-zero exit code).
-   `write` writes a file. `contents` is a string with the contents of the file
    to be written. You can also set `contentsBase64` to `true`, indicating that
    `contents` is a base64 encoded string it should decode before writing.
    `filename` is the path to the file to be written. This action doesn't
    manage permissions, nor ensures that the directory structure allows the
    file to be written. You should use an `exec` action to create directories,
    and to change permissions if needed. Largely, permissions can be managed by
    changing `user` to be the intended user.
-   `append` is exactly the same as `write`, but instead appends to the
    specified file.
-   `gobuild` builds Go binaries. `packages` is a list of Go packages to
    build, for example, `["grade"]` for the package (and binary) called
    `grade`. `ldflags` is a string that is added to the Go compiler's ldflags
    during build (mainly to set variables via the linker, without editing the
    source code on the fly). Currently, the assignment directory must contain
    a directory called `gosrc` which is considered to be `$GOPATH/src`, but
    this may change in the future. Additionally, it may be more helpful to be
    able to templetize the Go source instead of using the linker hack to add
    info, but that will wait until the future.
-   `parallel` executes many subactions in parallel. `subactions` is a list of
    actions. This is useful when needing to do many independent actions, like
    writing many files that don't require each other.
-   `ordered` executes many subactions in sequence. `subactions` is a list of
    actions. Paired with `parallel`, this can be used to construct more
    complicated parallel configurations.

`index.js` is run each time a spec instance is created. This means that
`index.js` can be changed on the server without needing to remove cached data.
Running the `generate` function takes so little time compared to managing
docker itself that this isn't a big problem.

All JS code that the server runs is protected against things like infinite
loops with timeouts.


## Legacy assignments

In older versions of uAssign, image building was controlled purely through
templetized Dockerfiles. The data sent with the specification is used as the
rendering context for the template, which is then sent to the docker daemon
as normal. If `index.js` isn't present in an assignment directory, then
the server will attempt to use the legacy method instead.

TODO: document these older assignment types.