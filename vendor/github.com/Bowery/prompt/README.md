# Prompt

[![Circle CI](https://circleci.com/gh/Bowery/prompt/tree/master.png?style=badge)](https://circleci.com/gh/Bowery/prompt/tree/master)

[![GoDoc](https://godoc.org/github.com/Bowery/prompt?status.png)](https://godoc.org/github.com/Bowery/prompt)

Prompt is a cross platform line-editing prompting library. Read the GoDoc page
for more info and for API details.

## Features
- Keyboard shortcuts in prompts
- History support
- Secure password prompt
- Custom prompt support
- Fallback prompt for unsupported terminals
- ANSI conversion for Windows

## Todo
- Multi-line prompt as a Terminal option
- Make refresh less jittery on Windows([possible reason](https://github.com/Bowery/prompt/blob/master/output_windows.go#L108))
- Multi-byte character support on Windows
- `AnsiWriter` should execute the equivalent ANSI escape code functionality on Windows
- Support for more ANSI escape codes on Windows.
- More keyboard shortcuts from Readlines shortcut list

## Contributing

Make sure Go is setup and running the latest release version, and make sure your `GOPATH` is setup properly.

Follow the guidelines [here](https://guides.github.com/activities/contributing-to-open-source/#contributing).

Please be sure to `gofmt` any code before doing commits. You can simply run `gofmt -w .` to format all the code in the directory.

Lastly don't forget to add your name to [`CONTRIBUTORS.md`](https://github.com/Bowery/prompt/blob/master/CONTRIBUTORS.md)

## License

Prompt is MIT licensed, details can be found [here](https://raw.githubusercontent.com/Bowery/prompt/master/LICENSE).
