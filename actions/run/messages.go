package run

type MessageTestRun struct {
	Filename string
	Source   string
	VUs      int
}

type MessageTestScale struct {
	VUs int
}

type MessageTestStop struct {
}
