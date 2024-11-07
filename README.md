# Indeedhat _stor (understore)
Like GNU Stow but dumber

## Install
```console
go install github.com/indeedhat/_stor@latest
```

## Usage
```
_stor
_stor provides a simple interface for creating, tracking and applying symlinks on your system to a common directory.
It is designed to allow you to track config files in a git repo, however that's not its only use.

Usage:
  _stor [flags]
  _stor [command]

Available Commands:
  apply       Apply the current _store repo to your system.
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  init        Initialize a new _stor repo here.
  release     Stop tracking a path in the _stor repo.
  track       Track a new path in the _stor repo.

Flags:
  -h, --help      help for _stor
  -v, --version   version for _stor

Use "_stor [command] --help" for more information about a command.
```

## Known Limitations
Due to the nature of how the db file works there is no history of old/removed entries this means that when calling apply
there is no way to clean up old symlinks from a previous db version.

Im not currently sure how or even if i want to approach this
