package utils

import (
    "fmt"
    "github.com/fatih/color"
)

func Failure(message string) string {
    red := color.New(color.FgRed).SprintFunc()
    return fmt.Sprintf("%v %v", message, red("failed"))
}

func Success(message string) string {
    green := color.New(color.FgGreen).SprintFunc()
    return fmt.Sprintf("%v %v", message, green("passed"))
}
