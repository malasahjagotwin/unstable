package protocol

const (
	BotTokenHeader  = "X-Bot-Token"
	StatusOK        = "ok"
	StatusFailed    = "failed"
	OutputTailBytes = 2048
)

type Task struct {
	ID     string   `json:"id"`
	Key    string   `json:"key"`
	Method string   `json:"method"`
	Host   string   `json:"host"`
	Time   int      `json:"time"`
	Argv   []string `json:"argv"`
}

type Report struct {
	ID       string `json:"id"`
	BotID    string `json:"bot_id"`
	Status   string `json:"status"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

func Tail(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}
