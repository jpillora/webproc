package agent

type msg struct {
	Pipe string `json:"p"`
	Buff string `json:"b"`
}

type msgQueuer struct {
	pipe  string
	queue chan msg
}

func (lq *msgQueuer) Write(p []byte) (int, error) {
	l := len(p)
	if l > 0 {
		lq.queue <- msg{lq.pipe, string(p)}
	}
	return l, nil
}
