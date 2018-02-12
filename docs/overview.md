# uAssign

uAssign (`ua`) is a system which allows terminal access to docker containers
over websockets, based on specifications. The uAssign server manages the
full lifetime of the containers, including building, running, and cleanup.

## How it works, end to end

0.  The service is setup. The client and server share a symmetric key,
which will be used to keep assignment information (like secrets/answers)
away from a student.

1.  A client builds a specification, and encrypts it using the shared key.
This specification includes information the server can use to build the
image. This information can vary depending on the requirements. The encrypted
data can be handed off safely to another user.

2.  A client (not necessarily the same as in part 1) sends the encrypted
specificiation to the server in order to create an "instance" (an instance
of a specification, with an associated container). The server produces
the instance by taking the encrypted specification and combining it with
scripts/templates, then building a docker image and container.

    - If, when given a specification, the server already has an instance
    of that specificiation, it will kick any users of that instance off
    (more later), and reuse that instance.
    - If there isn't an active instance, a new one is created, which
    consists mainly of a new docker image and container. When not in use,
    the container is stopped on the server (and removed after some time).

3.  The server gives the client back the instance's ID. The client now connects
to the server over a websocket, providing that instance ID.

    - If the instance isn't running, then the container is started.
    - If the instance is running, then the other connection is closed,
    and the incoming connection takes over.

4.  The server then runs a command on the container (specified in the spec),
and proxies stdin/stdout/stderr over the websocket (using a modified
[terminado](https://github.com/jupyter/terminado) protocol).

5.  When the client closes their connection, or no activity is seen for some
time, the container is stopped. This container can be returned to later.

    Additionally, a client can ask the server to clean up an instance, removing
it from the server. This can be used to preemptively clean up (for example,
when the instance is known to no longer be needed), or to reset things to
a fresh state to begin again.


The server manages instances over time by keeping track of their use. Once
an instance (and its image/container) are no longer needed, they will be
removed from the server. If a user requests a given instance again, it will
fail. However, a user can re-send a specification, and obtain a new instance.

## PrairieLearn integration

All of the work that involves a "client" is currently done through
PrairieLearn, though that's not a strict requirement. See
[prairielearn.md](prairielearn.md) for more info.