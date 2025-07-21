# Release

The package is used to release major version for framework and packages. The release process is very complex, so we built this package to save time.

## Usage

1. Set the `GITHUB_TOKEN` environment variable in the `.env` file to your GitHub token first.

Github link: https://github.com/settings/personal-access-tokens

Required accesses: Contents, Pull requests.

2. Preview the release process:

```
./artisan release major --framework=v1.16.0 --packages=v1.4.0
```

3. After previewing the release process, you can release the major version actually:

```
./artisan release major --framework=v1.16.0 --packages=v1.4.0 --real
```

## Testing

Run command below to run test:

```
go test ./...
```
