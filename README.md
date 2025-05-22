# hh-resume-auto-boost

A command-line tool to automatically boost your HeadHunter [(hh.ru)](https://hh.ru) resumes.

## Usage

1. Download the [latest release](https://github.com/ds8088/hh-resume-auto-boost/releases/latest);
2. Make a copy of `config.example.json` and save it as `config.json`;
3. Set your HeadHunter login and password in `config.json`;
4. Start the tool.

Alternatively, you may skip the configuration and run the tool with command-line arguments, such as:

`./hh-resume-auto-boost -l +78005553535 -p Bash1234`

However, this is not recommended due to the risk of exposing your password in the OS process list,
and furthermore, the password may be saved to your shell's command history,
so it's preferable to use the config file instead.

## How it works

The tool initially attempts to authenticate with HeadHunter using the provided credentials.

Assuming the authentication is successful, it will:

-   fetch the list of your resumes;
-   for each resume, schedule the boost as soon as possible, according to the HH boost interval (which is currently set to 4 hours);
-   keep the fetch/boost cycle running until you manually stop the program.

The tool also tries to masquerade itself as a generic, mainline Chrome browser
to avoid being marked as a bot.

## Disclaimer

This was just a quick and dirty way to increase the relevancy of my own resumes
that I wrote over the weekend.

Therefore, tests are unsurprisingly missing, the CI tooling is very basic,
the code quality seems a bit off, and containerization is left as an exercise
for the reader.
