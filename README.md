# Release

The package is used to release major and patch versions for framework and packages. The release process is very complex, so we built this package to save time.

## Setup

Set the `GITHUB_TOKEN` environment variable in the `.env` file to your GitHub token first. Required accesses: Contents, Pull requests.

Github link: https://github.com/settings/personal-access-tokens

## Usage

There are three main commands: `preview`, `major`, and `patch`.

1. Preview the release information

It's useful to preview the changes when generating the documentation.

```
# Preview framework and goravel-lite changes only
./artisan preview v1.16.0

# Preview all packages changes
./artisan preview v1.16.0 --packages
```

2. Release major version

The command will release the major version for framework and all sub-packages.

```
# Preview mode (default)
./artisan major v1.16.0

# Real release
./artisan major v1.16.0 --real

# With optional flags
./artisan major v1.16.0 --real --refresh --framework-branch=custom-branch
```

Available flags for major release:
- `--real`, `-r`: Perform actual release (without this flag, it's preview mode)
- `--refresh`: Refresh Go module proxy cache before release
- `--framework-branch`, `-fb`: Specify framework branch (useful when go mod cannot fetch the latest master)

3. Release patch version

The command will release the patch version for framework and example, goravel-lite, goravel.

```
# Preview mode (default)
./artisan patch v1.15.1

# Real release
./artisan patch v1.15.1 --real
```

## Testing

Run command below to run test:

```
go test ./...
```
