# hh-resume-auto-boost

A command-line tool to automatically boost your HeadHunter [(hh.ru)](https://hh.ru) resumes.

## Purpose

The idea behind the concept is very simple:

> In order for a resume to get noticed by a recruiter,
> one must use every tool at their disposal.

This includes boosting the resume periodically, preferably in an automated fashion.

Existing tools didn't work for me so that's pretty much all the motivation behind creating this project.

## Usage

1. Download the [latest release](https://github.com/ds8088/hh-resume-auto-boost/releases/latest);
2. Make a copy of `config.example.json` and save it as `config.json`;
3. Set your HeadHunter login and password in `config.json`;
4. Start the tool.

Alternatively, the configuration may be provided in the form of environment variables
(their names mirror the key names in config.json):

```sh
LOGIN=+78005553535
PASSWORD=Bash1234
```

You may also run the tool with command-line arguments, such as:

`./hh-resume-auto-boost -l +78005553535 -p Bash1234`

However, this is not recommended due to the risk of exposing your password in the OS process list,
and furthermore, the password may be saved to your shell's command history,
so it's preferable to use either the config file or environment variables instead.

All available keys and their accepted values are documented in the [config schema](.schema.json);
your IDE may automatically detect this file and, if so, both autocompletion and validation should work correctly.

## How it works

The tool initially attempts to authenticate with HeadHunter using the provided credentials.

Assuming the authentication is successful, it will:

- fetch the list of your resumes;
- for each resume, schedule the boost as soon as possible, according to the HH boost interval
  (which is currently set to 4 hours);
- keep the fetch/boost cycle running until you manually stop the program.

The tool also tries to masquerade itself as a generic, mainline Chrome browser
to avoid being marked as a bot.

## Docker image

A Docker image is available in GHCR.

To run the container, you should mount the directory that contains your
config.json to the container's /data.
The entire directory should be mounted so that the tool can persist cookies across restarts:

```sh
docker run -v ~/hh-resume-auto-boost:/data ghcr.io/ds8088/hh-resume-auto-boost:latest
```

As an alternative, you can run the container without a config file by passing
environment variables:

```sh
docker run -e LOGIN=+78005553535 -e PASSWORD=Bash1234 ghcr.io/ds8088/hh-resume-auto-boost:latest
```

CLI arguments are also supported (not recommended, because, as I mentioned previously,
the password will become exposed in the OS process list):

```sh
docker run ghcr.io/ds8088/hh-resume-auto-boost:latest -l +78005553535 -p Bash1234
```

## Building from source

Go 1.25+ is required.

```sh
go build ./...
```

To run unit tests:

```sh
go test -v ./...
```
