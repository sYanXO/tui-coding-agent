package logger

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	colorAgent   *color.Color
	colorTool    *color.Color
	colorUser    *color.Color
	colorSystem  *color.Color
	colorError   *color.Color
)

// Muted suppresses all logger output. Set to true when the TUI is active.
var Muted bool

func Init() {
	colorAgent = color.New(color.FgGreen)
	colorTool = color.New(color.FgCyan)
	colorUser = color.New(color.FgHiWhite, color.Bold)
	colorSystem = color.New(color.FgYellow)
	colorError = color.New(color.FgRed, color.Bold)
}

func Agent(format string, a ...interface{}) {
	if Muted {
		return
	}
	colorAgent.Printf(format+"\n", a...)
}

func AgentStream(text string) {
	if Muted {
		return
	}
	colorAgent.Print(text)
}

func Tool(format string, a ...interface{}) {
	if Muted {
		return
	}
	colorTool.Printf(format+"\n", a...)
}

func User(format string, a ...interface{}) {
	if Muted {
		return
	}
	colorUser.Printf(format+"\n", a...)
}

func System(format string, a ...interface{}) {
	if Muted {
		return
	}
	colorSystem.Printf(format+"\n", a...)
}

func Error(format string, a ...interface{}) {
	if Muted {
		return
	}
	colorError.Printf("ERROR: "+format+"\n", a...)
}

func Printf(format string, a ...interface{}) {
	if Muted {
		return
	}
	fmt.Printf(format, a...)
}

func DiffHeader(format string, a ...interface{}) {
	color.New(color.FgCyan).Printf(format+"\n", a...)
}

func DiffMinus(format string, a ...interface{}) {
	color.New(color.FgRed).Printf(format+"\n", a...)
}

func DiffPlus(format string, a ...interface{}) {
	color.New(color.FgGreen).Printf(format+"\n", a...)
}

func Tokens(line string) {
	color.New(color.FgHiBlack).Println(line)
}

func TokenSummary(summary string) {
	color.New(color.FgYellow).Println(summary)
}
