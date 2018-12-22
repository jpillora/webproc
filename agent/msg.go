package agent

import "bytes"

type msg struct {
	Pipe string `json:"p"`
	Buff string `json:"b"`
}

type msgQueuer struct {
	pipe  string
	queue chan msg
}

func (lq *msgQueuer) Write(data []byte) (int, error) {
	data = stripAnsi(data)
	l := len(data)
	if l > 0 {
		lines := bytes.Split(data, []byte("\n"))
		lastIndex := len(lines) - 1
		for i, d := range lines {
			line := string(d)
			if i != lastIndex {
				line += "\n"
			}
			if line == "" {
				continue
			}
			lq.queue <- msg{lq.pipe, line}
		}
	}
	return l, nil
}
