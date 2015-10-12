func (target *TargetFile) RenderDiff() ([]byte, error) {
	fromPath := target.PathIn(common.ProvisionedDirectory())
	toPath := target.PathIn(common.TargetDirectory())
func getDiffBody(fromPath, toPath, reportedPath string) []byte {
	errorReport := common.Report{Action: "diff", Target: toPath}
	output, _ := common.ExecProgram(&errorReport, []byte{}, "diff", "-u", fromPath, toPath)
	errorReport.PrintUnlessEmpty()
	return append(header, getDiffBody("/dev/null", path, reportedPath)...)
	diffBody := getDiffBody(fromPath, toPath, reportedPath)
	return append(header, getDiffBody(path, "/dev/null", reportedPath)...)