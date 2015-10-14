/*******************************************************************************
*
* Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
*
* This file is part of Holo.
*
* Holo is free software: you can redistribute it and/or modify it under the
* terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* Holo is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* Holo. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package files

import (
	"fmt"

	"../../shared"
)

//The stuff in this file used to be inside src/holo/apply.go, but it was split
//to emphasize the standardized interface of application implementations.

//ApplyImpl is the return type for GetApplyImpl.
type ApplyImpl func(*FileBuffer, *shared.Report) (*FileBuffer, error)

//GetApplyImpl returns a function that applies the given RepoFile to a file
//buffer, as part of the `holo apply` algorithm.
func GetApplyImpl(repoFile RepoFile) ApplyImpl {
	var impl func(RepoFile, *FileBuffer, *shared.Report) (*FileBuffer, error)
	if repoFile.ApplicationStrategy() == "passthru" {
		impl = applyScript
	} else {
		impl = applyFile
	}
	return func(fb *FileBuffer, r *shared.Report) (*FileBuffer, error) {
		return impl(repoFile, fb, r)
	}
}

func applyFile(repoFile RepoFile, buffer *FileBuffer, report *shared.Report) (*FileBuffer, error) {
	//if the repo contains a plain file (or symlink), the file
	//buffer is replaced by it, thus ignoring the target base (or any
	//previous application steps)
	return NewFileBuffer(repoFile.Path(), buffer.BasePath)
}

func applyScript(repoFile RepoFile, buffer *FileBuffer, report *shared.Report) (*FileBuffer, error) {
	//this application strategy requires file contents
	buffer, err := buffer.ResolveSymlink()
	if err != nil {
		return nil, err
	}

	//run command, fetch result file into buffer (not into the targetPath
	//directly, in order not to corrupt the file there if the script run fails)
	output, err := shared.ExecProgram(report, buffer.Contents, repoFile.Path())
	if err != nil {
		return nil, fmt.Errorf("execution of %s failed: %s", repoFile.Path(), err.Error())
	}

	//result is the stdout of the script
	return NewFileBufferFromContents(output, buffer.BasePath), nil
}
