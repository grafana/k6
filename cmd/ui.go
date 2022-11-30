package cmd

import "go.k6.io/k6/ui/console/pb"

func maybePrintBanner(gs *globalState) {
	if !gs.flags.quiet {
		gs.console.Printf("\n%s\n\n", gs.console.Banner())
	}
}

func maybePrintBar(gs *globalState, bar *pb.ProgressBar) {
	if !gs.flags.quiet {
		gs.console.PrintBar(bar)
	}
}
