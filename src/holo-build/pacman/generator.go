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

package pacman

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"../../shared"
	"../common"
)

//Generator is the common.Generator for Pacman packages (as used by Arch Linux
//and derivatives).
type Generator struct{}

//RecommendedFileName implements the common.Generator interface.
func (g *Generator) RecommendedFileName(pkg *common.Package) string {
	//this is called after Build(), so we can assume that package name,
	//version, etc. were already validated
	return fmt.Sprintf("%s-%s-any.pkg.tar.xz", pkg.Name, pkg.Version)
}

//Build implements the common.Generator interface.
func (g *Generator) Build(pkg *common.Package, rootPath string, buildReproducibly bool) ([]byte, error) {
	//TODO: validate package names, versions

	//write .PKGINFO
	err := writePKGINFO(pkg, rootPath, buildReproducibly)
	if err != nil {
		return nil, fmt.Errorf("Failed to write .PKGINFO: %s", err.Error())
	}

	//write .INSTALL
	err = writeINSTALL(pkg, rootPath, buildReproducibly)
	if err != nil {
		return nil, fmt.Errorf("Failed to write .INSTALL: %s", err.Error())
	}

	//write mtree
	err = writeMTREE(rootPath, buildReproducibly)
	if err != nil {
		return nil, fmt.Errorf("Failed to write .MTREE: %s", err.Error())
	}

	//compress package
	return compressPackage(rootPath)
}

func fullVersionString(pkg *common.Package) string {
	//pkg.Version may not contain dashes in pacman packages, so replace "-" with "_"
	str := fmt.Sprintf("%s-%d", strings.Replace(pkg.Version, "-", "_", -1), pkg.Release)
	if pkg.Epoch > 0 {
		str = fmt.Sprintf("%d:%s", pkg.Epoch, str)
	}
	return str
}

func writePKGINFO(pkg *common.Package, rootPath string, buildReproducibly bool) error {
	//gather metrics
	installedSize, err := findPackageInstalledSize(rootPath)
	if err != nil {
		return err
	}

	//get fakeroot version
	fakerootVersionString, err := exec.Command("fakeroot", "--version").Output()
	if err != nil {
		return err
	}

	//normalize package description like makepkg does
	desc := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(pkg.Description), " ")

	//generate .PKGINFO
	contents := ""
	if buildReproducibly {
		contents = "# Generated by holo-build in reproducible mode\n"
	} else {
		contents = fmt.Sprintf("# Generated by holo-build %s\n", shared.Version())
		contents += fmt.Sprintf("# using %s\n", strings.TrimSpace(string(fakerootVersionString)))
	}
	contents += fmt.Sprintf("pkgname = %s\n", pkg.Name)
	contents += fmt.Sprintf("pkgver = %s\n", fullVersionString(pkg))
	contents += fmt.Sprintf("pkgdesc = %s\n", desc)
	contents += "url = \n"
	if !buildReproducibly {
		contents += fmt.Sprintf("builddate = %d\n", time.Now().Unix())
	}
	contents += "packager = Unknown Packager\n"
	contents += fmt.Sprintf("size = %d\n", installedSize)
	contents += "arch = any\n"
	contents += "license = custom:none\n"
	contents += compilePackageRelations("replaces", pkg.Replaces)
	contents += compilePackageRelations("conflict", pkg.Conflicts)
	contents += compilePackageRelations("provides", pkg.Provides)
	contents += compilePackageRelations("depend", pkg.Requires)

	//we used holo-build to build this, so the build depends on the package
	//"holo" which contains holo-build
	contents += "makedepend = holo\n"
	//these makepkgopt are fabricated (well, duh) and describe the behavior of
	//holo-build in terms of these options
	contents += "makepkgopt = !strip\n"
	contents += "makepkgopt = docs\n"
	contents += "makepkgopt = libtool\n"
	contents += "makepkgopt = staticlibs\n"
	contents += "makepkgopt = emptydirs\n"
	contents += "makepkgopt = !zipman\n"
	contents += "makepkgopt = !purge\n"
	contents += "makepkgopt = !upx\n"
	contents += "makepkgopt = !debug\n"

	//write .PKGINFO
	return common.WriteFile(filepath.Join(rootPath, ".PKGINFO"), []byte(contents), 0644, buildReproducibly)
}

//Returns the installed size of the package (in bytes).
func findPackageInstalledSize(rootPath string) (int, error) {
	//we use the same method as makepkg, which is `du -s --apparent-size`
	cmd := exec.Command("du", "-s", "-B", "1", "--apparent-size", rootPath)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	//output is size in bytes + "\t" + path
	match := regexp.MustCompile(`^([0-9]+)\s`).FindSubmatch(output)
	if match == nil {
		return 0, fmt.Errorf("invalid output returned from `du -s -B 1 --apparent-size %s`: \"%s\"", rootPath, string(output))
	}
	return strconv.Atoi(string(match[1]))
}

//Renders package relations into .PKGINFO.
func compilePackageRelations(relType string, rels []common.PackageRelation) string {
	if len(rels) == 0 {
		return ""
	}

	lines := make([]string, 0, len(rels)) //only a lower boundary on the final size, but usually a good guess
	for _, rel := range rels {
		if len(rel.Constraints) == 0 {
			//simple relation without constraint, e.g. "depend = linux"
			lines = append(lines, fmt.Sprintf("%s = %s", relType, rel.RelatedPackage))
		} else {
			for _, c := range rel.Constraints {
				//relation with constraint, e.g. "conflict = holo<0.5"
				lines = append(lines, fmt.Sprintf("%s = %s%s%s", relType, rel.RelatedPackage, c.Relation, c.Version))
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func writeINSTALL(pkg *common.Package, rootPath string, buildReproducibly bool) error {
	//assemble the contents for the .INSTALL file
	contents := ""
	if script := strings.TrimSpace(pkg.SetupScript); script != "" {
		contents += fmt.Sprintf("post_install() {\n%s\n}\npost_upgrade() {\npost_install\n}\n", script)
	}
	if script := strings.TrimSpace(pkg.CleanupScript); script != "" {
		contents += fmt.Sprintf("post_remove() {\n%s\n}\n", script)
	}

	//do we need the .INSTALL file at all?
	if contents == "" {
		return nil
	}

	return common.WriteFile(filepath.Join(rootPath, ".INSTALL"), []byte(contents), 0644, buildReproducibly)
}

func writeMTREE(rootPath string, buildReproducibly bool) error {
	//list all desired entries in rootPath
	entries, err := filepath.Glob(filepath.Join(rootPath, "*"))
	if err != nil {
		return err
	}
	targets := make([]string, 0, len(entries))
	for _, entry := range entries {
		target, err := filepath.Rel(rootPath, entry)
		if err != nil {
			return err
		}
		//filepath.Rel() may result in "foo" instead of "./foo" sometimes, but
		//makepkg has .MTREEs with "./foo" paths, so enforce that
		if !strings.HasPrefix(target, "./") {
			target = "./" + target
		}
		targets = append(targets, target)
	}

	//generate mtree data
	cmd := exec.Command(
		//using standardized language settings...
		"env", append([]string{"LANG=C",
			//...generate an archive...
			"bsdtar", "-czf", ".MTREE",
			//...in mtree format with only the required filesystem metadata
			"--format=mtree", "--options=!all,use-set,type,uid,gid,mode,time,size,md5,sha256,link",
			//of these things
		}, targets...)...,
	)
	cmd.Dir = rootPath
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	err = os.Chmod(filepath.Join(rootPath, ".MTREE"), 0644)
	if err != nil {
		return err
	}

	if buildReproducibly {
		return common.ResetTimestamp(filepath.Join(rootPath, ".MTREE"))
	}
	return nil
}

func compressPackage(rootPath string) ([]byte, error) {
	cmd := exec.Command(
		//using standardized language settings...
		"env", "LANG=C",
		//...generate a .tar.xz archive...
		"bsdtar", "-cJf", "-",
		//...with the leading "./" path element stripped...
		"--strip-components", "1",
		//...of the working directory (== rootPath)
		".",
	)
	cmd.Dir = rootPath
	cmd.Stderr = os.Stderr
	return cmd.Output()
}
