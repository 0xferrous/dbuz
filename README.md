# dbus-debug

`dbus-debug` is a terminal UI for exploring and interacting with D-Bus without manually crafting `busctl` or `gdbus` commands.

It lets you browse buses, services, object paths, interfaces, methods, properties, and signals in a stacked-pane interface.

## Run

```sh
go run .
```

Or with Nix:

```sh
nix run
```

## Keybindings

- `↑/k`, `↓/j`: move selection
- `→/l/enter`: dive into the selected item
- `←/h/backspace`: go back
- `pgup` / `ctrl+u`: page up
- `pgdown` / `ctrl+d`: page down
- `[` / `]`: scroll the debug log
- `q` / `ctrl+c`: quit

## D-Bus mental model

D-Bus is local inter-process communication. A useful way to think about it is:

```text
bus → bus name/service → object path → interface → method/property/signal
```

A full method call target is made from:

```text
bus:         system or session
bus name:    org.freedesktop.NetworkManager
object path: /org/freedesktop/NetworkManager
interface:   org.freedesktop.NetworkManager
method:      GetDevices
args:        ...
```

So unlike many JSON-RPC APIs where you call a method name on one endpoint, D-Bus calls are addressed to a service, object path, interface, and method.

## Buses

A D-Bus bus is a local message broker/router.

Common buses:

- **session bus**: per-user applications and desktop services
- **system bus**: system services like NetworkManager, systemd, logind, Bluetooth, etc.

Apps connect to a bus, claim names, send method calls, emit signals, and expose objects.

## Bus names / services

A bus name identifies a service on a bus.

Examples:

```text
org.freedesktop.NetworkManager
org.freedesktop.systemd1
org.freedesktop.Notifications
:1.42
```

There are two broad kinds:

- **well-known names**: stable names like `org.freedesktop.NetworkManager`
- **unique names**: connection-specific names like `:1.42`

Well-known names are usually what humans care about. Unique names identify a specific connection to the bus.

`dbus-debug` shows metadata for selected bus names when available:

- kind: `well-known` or `unique`
- whether the name is activatable
- current owner unique name for well-known names
- Unix process ID of the owning connection
- Unix user ID of the owning connection

This metadata comes from standard `org.freedesktop.DBus` calls:

```text
ListActivatableNames
GetNameOwner
GetConnectionUnixProcessID
GetConnectionUnixUser
```

## Object paths and objects

A D-Bus service exposes objects at object paths.

Examples:

```text
/
/org/freedesktop/NetworkManager
/org/freedesktop/systemd1/unit/ssh_2eservice
```

Object paths look like filesystem paths, but they are not files. They are addresses for objects inside a D-Bus service.

An object can have:

- child object paths
- interfaces
- methods
- properties
- signals

## Interfaces

An interface groups related methods, properties, and signals.

Examples:

```text
org.freedesktop.DBus.Introspectable
org.freedesktop.DBus.Properties
org.freedesktop.NetworkManager
```

Interfaces prevent name collisions and provide structure. A single object can implement many interfaces.

## Methods

Methods are callable operations.

Example method shape:

```text
GetDevices() → ao
```

This means:

- no input args
- returns `ao`, an array of object paths

A method may be read-like or mutating, but D-Bus introspection usually does not reliably say which. Treat method calls as potentially side-effectful unless you know the API.

## Properties

Properties are named values exposed by an interface.

They have:

- type
- access mode

Access modes:

- `read`
- `write`
- `readwrite`

Properties are usually accessed through:

```text
org.freedesktop.DBus.Properties.Get
org.freedesktop.DBus.Properties.Set
org.freedesktop.DBus.Properties.GetAll
```

Many services also emit `PropertiesChanged` signals when values change.

## Signals

Signals are events emitted by services.

They are like subscriptions/events/logs. A client can listen for signals matching a sender, object path, interface, and signal name.

Example:

```text
PropertiesChanged(s, a{sv}, as)
```

This signal commonly means some properties changed.

## Introspection

D-Bus services can expose XML metadata through:

```text
org.freedesktop.DBus.Introspectable.Introspect
```

This tells clients what exists at an object path:

- child nodes
- interfaces
- methods
- method args
- properties
- signals
- annotations

`dbus-debug` uses introspection to populate panes.

Caveats:

- some services expose incomplete introspection
- some object paths may not introspect
- arg names are optional and may be missing
- method side effects are generally not marked

## D-Bus type signatures

D-Bus uses compact type signatures.

Primitive types:

| Code | Meaning |
|---|---|
| `y` | byte |
| `b` | boolean |
| `n` | int16 |
| `q` | uint16 |
| `i` | int32 |
| `u` | uint32 |
| `x` | int64 |
| `t` | uint64 |
| `d` | double |
| `h` | Unix file descriptor |
| `s` | string |
| `o` | object path |
| `g` | signature |
| `v` | variant |

Container types:

| Signature | Meaning |
|---|---|
| `as` | array of strings |
| `ao` | array of object paths |
| `a{sv}` | dictionary from string to variant |
| `(su)` | struct of string and uint32 |
| `a(su)` | array of structs |

Common examples:

```text
s             string
ao            array<object path>
as            array<string>
a{sv}         dictionary<string, variant>
(su)          struct<string, uint32>
(sa{sv}as)    struct<string, dictionary<string, variant>, array<string>>
```

`dbus-debug` decodes these signatures in metadata so you do not have to remember them.

## Variants

`v` means variant: a value that carries its own runtime type.

D-Bus APIs often use variants in dictionaries:

```text
a{sv}
```

This means:

```text
dictionary<string, variant>
```

It is common for property maps and option maps.

## Annotations

Annotations are extra metadata in introspection XML.

They may describe things like:

- deprecation
- emitted property-change behavior
- language/tool-specific metadata
- custom service-specific hints

Annotations are optional and not always standardized. `dbus-debug` shows annotations when present.

## Debug log

The bottom log pane shows timestamped D-Bus operations made by the TUI, such as:

- bus connection attempts
- `ListNames` calls
- introspection calls
- replies/errors

Use `[` and `]` to scroll the log.

## Current capabilities

Implemented:

- session/system bus selection
- bus name browsing
- bus name metadata: kind, activatable status, owner, PID, UID
- object path browsing
- interface browsing
- method/property/signal browsing
- introspection parsing
- D-Bus type decoding
- selected-item metadata
- breadcrumb/status line
- debug log pane

Planned:

- property get/set
- method calling
- signal subscription
- search/filtering
- generated `busctl` commands
- call/result history
