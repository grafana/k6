var i = $vu.iteration();
$log.info("Iteration", {i: i});
if (i == 10) {
	$test.abort();
}
