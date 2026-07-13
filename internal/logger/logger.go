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

func Init() {
	colorAgent = color.New(color.FgGreen)
	colorTool = color.New(color.FgCyan)
	colorUser = color.New(color.FgHiWhite, color.Bold)
	colorSystem = color.New(color.FgYellow)
	colorError = color.New(color.FgRed, color.Bold)
}

func Agent(format string, a ...interface{}) {
	colorAgent.Printf(format+"\n", a...)
}

func AgentStream(text string) {
	colorAgent.Print(text)
}

func Tool(format string, a ...interface{}) {
	colorTool.Printf(format+"\n", a...)
}

func User(format string, a ...interface{}) {
	colorUser.Printf(format+"\n", a...)
}

func System(format string, a ...interface{}) {
	colorSystem.Printf(format+"\n", a...)
}

func Error(format string, a ...interface{}) {
	colorError.Printf("ERROR: "+format+"\n", a...)
}

func Printf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}
