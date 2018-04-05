# PrairieLearn integration

uAssign's (current) primary frontend is PrairieLearn, consisting of:

-   A python library for `serverFilesCourse` which abstracts away the spec's
representation.
-   A v3 element (`uassign_terminal`), which renders the terminal for the
browser.


## Python library for question generation

uAssign questions are done in PrairieLearn using the v3 question type. The
`uassign` library can be imported to generate specifications, encrypt them,
as well as invoke cleanups on the uAssign server. For example:

```python
import uassign

...

def generate(data):
    assignment_name = data['options']['assignmentName']
    filenames = data['options']['filenames']
    filename = random.choice(filenames)

    ...

    spec = uassign.create_spec(assignment_name, {
        'files': [
            {
                'filename': filename,
                'contents': contents,
            },
        ],
    })

    data['params']['spec'] = spec
    data['params']['filename'] = filename

```

In this example, we import the `uassign` Python library, then use it to
create a specification, which is just a dictionary-style object containing
information the server can use to instantiate a spec (for example, some
number of files and file contents). The library returns a string, which
is sent to the server when working with the spec. `assignment_name` is the name
of the assignment known by the uAssign server, for example,
`archive.tar_extract`.

The string is a JSON object, which includes an encrypted payload and
associated HMAC. The server will verify the payload, and pull the encrypted
object out via JSON. Currently, the AES key used to encrypted the payload is
hardcoded in the `uassign` library, but this may change in the future.


## `uassign_terminal` element

The `uassign_terminal` element handles the rendering/management of the
terminal. In `question.html`, this only requires adding:

```html
<uassign_terminal spec_name="spec" />
```

Where `spec_name` is the name of the spec data in the `params` dictionary
as shown above. This mirrors other elements which read parameters.

In addition to `spec_name` (which is required), `uassign_terminal` has other
options:

-   `host`, which changes the hostname of the uAssign server, for example:
    `host="uassign.example.edu"`
-   `insecure`, which when set to true (`insecure="true"` or similar) will
    use normal websockets instead of secure websockets.
