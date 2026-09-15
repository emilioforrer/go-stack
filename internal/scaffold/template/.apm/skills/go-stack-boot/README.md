# go-stack-boot skill v1.2.0

Updated skill bundle implementing the lifecycle/composition recommendations:

- boot explicitly defined as a thin executable composition entry point;
- packages and services are both supported lifecycle participants;
- providers may resolve application dependencies through shared DI while normal
  runtime code does not use the injector as a service locator;
- constructors construct, Register wires, Boot activates, Shutdown releases;
- application-lifetime context semantics are explicit;
- critical post-Boot failures require supervision;
- libraries/SDKs are explicitly outside the normal boot boundary;
- example refactored so HTTP/worker behavior does not live in provider files.
