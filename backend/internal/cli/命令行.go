package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

/*
New 返回一个静默的 flag.FlagSet：解析失败时不打印任何内容，
由调用方用本包的中文文案统一输出。
*/
func New(name string) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	return set
}

/*
Parse 解析命令行参数。Go 的 flag 包会把未知标志直接判定为
"flag provided but not defined" 并写入英文提示，因此这里先自行
校验标志名，好让未知标志能够以中文报错。
*/
func Parse(set *flag.FlagSet, args []string) error {
	for _, arg := range args {
		if arg == "--help" || arg == "-help" || arg == "-h" {
			return flag.ErrHelp
		}
	}

	if unknown := firstUnknownFlag(set, args); unknown != "" {
		return fmt.Errorf("提供了未定义的标志：%s", unknown)
	}
	return set.Parse(args)
}

/*
Usage 打印中文的帮助信息，格式为「用法：」加每个标志的说明，
并用全角括号补上非零的默认值。
*/
func Usage(set *flag.FlagSet, w io.Writer) {
	fmt.Fprintln(w, "用法：")
	set.VisitAll(func(entry *flag.Flag) {
		if entry.Name == "" {
			return
		}
		typeName, description := flag.UnquoteUsage(entry)
		fmt.Fprintf(w, "  -%s", entry.Name)
		if typeName != "" {
			fmt.Fprintf(w, " %s", typeName)
		}
		fmt.Fprintln(w)

		fmt.Fprintf(w, "    \t%s", description)
		if hasDefault(entry) {
			fmt.Fprintf(w, "（默认 %s）", entry.DefValue)
		}
		fmt.Fprintln(w)
	})
}

func hasDefault(entry *flag.Flag) bool {
	switch entry.DefValue {
	case "", "0", "false", "<nil>":
		return false
	default:
		return true
	}
}

func firstUnknownFlag(set *flag.FlagSet, args []string) string {
	for _, arg := range args {
		if arg == "--" {
			return ""
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return ""
		}

		name := strings.TrimLeft(arg, "-")
		if before, _, found := strings.Cut(name, "="); found {
			name = before
		}
		if name == "" {
			return ""
		}
		if set.Lookup(name) == nil {
			return "-" + name
		}
	}
	return ""
}
