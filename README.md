# redminefs

Redmine filesystem written in Go

See it in action: https://asciinema.org/a/18038

## Why

The initial idea is to manipulate redmine issues in an easy way, just like files and folders.

## Disclaimer

This project is under light development and should not be used in any environment.

Pull requests are welcomed.

## Usage

* Put a setting file in `~/config/godmine/settings.json`:

```
{
    "endpoint": "https://example.redmine.com/",
    "apikey": "apikey"
}
```

* Check out the source code and run `go build`

* Run `./redminefs <mount point>` to mount your redminefs.
