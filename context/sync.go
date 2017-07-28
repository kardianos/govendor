// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/vendorfile"

	"golang.org/x/tools/go/vcs"
)

func skipperTree(name string, dir bool) bool {
	return false
}
func skipperPackage(name string, dir bool) bool {
	return dir
}

func (ctx *Context) VerifyVendor() (outOfDate []*vendorfile.Package, err error) {
	vf := ctx.VendorFile
	root := filepath.Join(ctx.RootDir, ctx.VendorFolder)
	add := func(vp *vendorfile.Package) {
		outOfDate = append(outOfDate, vp)
	}
	for _, vp := range vf.Package {
		if vp.Remove {
			continue
		}
		if len(vp.Path) == 0 {
			continue
		}
		if len(vp.ChecksumSHA1) == 0 {
			add(vp)
			continue
		}
		fp := filepath.Join(root, pathos.SlashToFilepath(vp.Path))
		h := sha1.New()
		sk := skipperPackage
		if vp.Tree {
			sk = skipperTree
		}
		err = getHash(root, fp, h, sk)
		if err != nil {
			return
		}
		checksum := base64.StdEncoding.EncodeToString(h.Sum(nil))
		if vp.ChecksumSHA1 != checksum {
			add(vp)
		}
	}
	return
}

func getHash(root, fp string, h hash.Hash, skipper func(name string, isDir bool) bool) error {
	rel := pathos.FileTrimPrefix(fp, root)
	rel = pathos.SlashToImportPath(rel)
	rel = strings.Trim(rel, "/")

	h.Write([]byte(rel))

	dir, err := os.Open(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Failed to open dir %q: %v", fp, err)
	}
	filelist, err := dir.Readdir(-1)
	dir.Close()
	if err != nil {
		return fmt.Errorf("Failed to read dir %q: %v", fp, err)
	}
	sort.Sort(fileInfoSort(filelist))
	for _, fi := range filelist {
		if skipper(fi.Name(), fi.IsDir()) {
			continue
		}
		p := filepath.Join(fp, fi.Name())
		if fi.IsDir() {
			err = getHash(root, p, h, skipper)
			if err != nil {
				return err
			}
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		h.Write([]byte(fi.Name()))
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// similarSegments compares two paths and determines if they have
// similar prefix segments. For example github.com/kardianos/rdb and
// github.com/kardianos/govendor have 2 similar segments.
func similarSegments(p1, p2 string) (string, int) {
	seg1 := strings.Split(p1, "/")
	seg2 := strings.Split(p2, "/")

	ct := len(seg1)
	if len(seg2) < ct {
		ct = len(seg2)
	}

	similar := &bytes.Buffer{}
	for i := 0; i < ct; i++ {
		if seg1[i] != seg2[i] {
			return similar.String(), i
		}
		if i != 0 {
			similar.WriteRune('/')
		}
		similar.WriteString(seg1[i])
	}
	return similar.String(), ct
}

type remoteFailure struct {
	Path string
	Msg  string
	Err  error
}

func (fail remoteFailure) Error() string {
	return fmt.Sprintf("Failed for %q (%s): %v", fail.Path, fail.Msg, fail.Err)
}

type remoteFailureList []remoteFailure

func (list remoteFailureList) Error() string {
	if len(list) == 0 {
		return "(no remote failure)"
	}
	buf := &bytes.Buffer{}
	buf.WriteString("Remotes failed for:\n")
	for _, item := range list {
		buf.WriteString("\t")
		buf.WriteString(item.Error())
		buf.WriteString("\n")
	}
	return buf.String()
}

type VCSCmd struct {
	*vcs.Cmd
}

func (vcsCmd *VCSCmd) RevisionSync(dir, revision string) error {
	return vcsCmd.run(dir, vcsCmd.TagSyncCmd, "tag", revision)
}

func (v *VCSCmd) run(dir string, cmd string, keyval ...string) error {
	_, err := v.run1(dir, cmd, keyval, true)
	return err
}

// run1 is the generalized implementation of run and runOutput.
func (vcsCmd *VCSCmd) run1(dir string, cmdline string, keyval []string, verbose bool) ([]byte, error) {
	v := vcsCmd.Cmd
	m := make(map[string]string)
	for i := 0; i < len(keyval); i += 2 {
		m[keyval[i]] = keyval[i+1]
	}
	args := strings.Fields(cmdline)
	for i, arg := range args {
		args[i] = expand(m, arg)
	}

	_, err := exec.LookPath(v.Cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"go: missing %s command. See http://golang.org/s/gogetcmd\n",
			v.Name)
		return nil, err
	}

	cmd := exec.Command(v.Cmd, args...)
	cmd.Dir = dir
	cmd.Env = envForDir(cmd.Dir)
	if vcs.ShowCmd {
		fmt.Printf("cd %s\n", dir)
		fmt.Printf("%s %s\n", v.Cmd, strings.Join(args, " "))
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	out := buf.Bytes()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "# cd %s; %s %s\n", dir, v.Cmd, strings.Join(args, " "))
			os.Stderr.Write(out)
		}
		return nil, err
	}
	return out, nil
}

// expand rewrites s to replace {k} with match[k] for each key k in match.
func expand(match map[string]string, s string) string {
	for k, v := range match {
		s = strings.Replace(s, "{"+k+"}", v, -1)
	}
	return s
}

// envForDir returns a copy of the environment
// suitable for running in the given directory.
// The environment is the current process's environment
// but with an updated $PWD, so that an os.Getwd in the
// child will be faster.
func envForDir(dir string) []string {
	env := os.Environ()
	// Internally we only use rooted paths, so dir is rooted.
	// Even if dir is not rooted, no harm done.
	return mergeEnvLists([]string{"PWD=" + dir}, env)
}

// mergeEnvLists merges the two environment lists such that
// variables with the same name in "in" replace those in "out".
func mergeEnvLists(in, out []string) []string {
NextVar:
	for _, inkv := range in {
		k := strings.SplitAfterN(inkv, "=", 2)[0]
		for i, outkv := range out {
			if strings.HasPrefix(outkv, k) {
				out[i] = inkv
				continue NextVar
			}
		}
		out = append(out, inkv)
	}
	return out
}

func updateVcsCmd(cmd *vcs.Cmd) *VCSCmd {
	switch cmd.Name {
	case "Git":
		cmd.TagSyncCmd = "reset --hard {tag}"
		cmd.TagSyncDefault = "reset --hard origin/master"
		cmd.DownloadCmd = "fetch"
	case "Mercurial":
	case "Bazaar":
	case "Subversion":
	}
	return &VCSCmd{Cmd: cmd}
}

var isSecureScheme = map[string]bool{
	"https":   true,
	"git+ssh": true,
	"bzr+ssh": true,
	"svn+ssh": true,
	"ssh":     true,
}

func vcsIsSecure(repo string) bool {
	u, err := url.Parse(repo)
	if err != nil {
		// If repo is not a URL, it's not secure.
		return false
	}
	return isSecureScheme[u.Scheme]
}

// Sync checks for outdated packages in the vendor folder and fetches the
// correct revision from the remote.
func (ctx *Context) Sync(dryrun bool) (err error) {
	// vcs.ShowCmd = true
	outOfDate, err := ctx.VerifyVendor()
	if err != nil {
		return fmt.Errorf("Failed to verify checksums: %v", err)
	}
	// GOPATH includes the src dir, move up a level.
	cacheRoot := filepath.Join(ctx.RootGopath, "..", ".cache", "govendor")
	err = os.MkdirAll(cacheRoot, 0700)
	if err != nil {
		return err
	}

	// collect errors and proceed where you can.
	rem := remoteFailureList{}

	h := sha1.New()
	updatedVendorFile := false

	for _, vp := range outOfDate {
		// Bundle packages together that have the same revision and share at least one root segment.
		if len(vp.Revision) == 0 {
			continue
		}
		from := vp.Path
		if len(vp.Origin) > 0 {
			from = vp.Origin
		}
		if from != vp.Path {
			fmt.Fprintf(ctx, "fetch %q from %q\n", vp.Path, from)
		} else {
			fmt.Fprintf(ctx, "fetch %q\n", vp.Path)
		}
		if dryrun {
			continue
		}
		pkgDir := filepath.Join(cacheRoot, from)

		// See if repo exists.
		sysVcsCmd, repoRoot, err := vcs.FromDir(pkgDir, cacheRoot)
		var vcsCmd *VCSCmd
		repoRootDir := filepath.Join(cacheRoot, repoRoot)
		if err != nil {
			rr, err := vcs.RepoRootForImportPath(from, false)
			if err != nil {
				rem = append(rem, remoteFailure{Msg: "failed to ping remote repo", Path: vp.Path, Err: err})
				continue
			}
			if !ctx.Insecure && !vcsIsSecure(rr.Repo) {
				rem = append(rem, remoteFailure{Msg: "repo remote not secure", Path: vp.Path, Err: nil})
				continue
			}

			vcsCmd = updateVcsCmd(rr.VCS)

			repoRoot = rr.Root
			repoRootDir = filepath.Join(cacheRoot, repoRoot)
			err = os.MkdirAll(repoRootDir, 0700)
			if err != nil {
				rem = append(rem, remoteFailure{Msg: "failed to make repo root dir", Path: vp.Path, Err: err})
				continue
			}

			err = vcsCmd.CreateAtRev(repoRootDir, rr.Repo, vp.Revision)
			if err != nil {
				rem = append(rem, remoteFailure{Msg: "failed to clone repo", Path: vp.Path, Err: err})
				continue
			}
		} else {
			// Use cache.
			vcsCmd = updateVcsCmd(sysVcsCmd)

			err = vcsCmd.RevisionSync(repoRootDir, vp.Revision)
			// If revision was not found in the cache, download and try again.
			if err != nil {
				err = vcsCmd.Download(repoRootDir)
				if err != nil {
					rem = append(rem, remoteFailure{Msg: "failed to download repo", Path: vp.Path, Err: err})
					continue
				}
				err = vcsCmd.RevisionSync(repoRootDir, vp.Revision)
				if err != nil {
					rem = append(rem, remoteFailure{Msg: "failed to sync repo to " + vp.Revision, Path: vp.Path, Err: err})
					continue
				}
			}
		}
		dest := filepath.Join(ctx.RootDir, ctx.VendorFolder, pathos.SlashToFilepath(vp.Path))
		// Path handling with single sub-packages and differing origins need to be properly handled.
		src := pkgDir

		// Scan go files for files that should be ignored based on tags and filenames.
		ignoreFiles, _, err := ctx.getIgnoreFiles(src)
		if err != nil {
			rem = append(rem, remoteFailure{Msg: "failed to get ignore files", Path: vp.Path, Err: err})
			continue
		}

		root, _ := pathos.TrimCommonSuffix(src, vp.Path)

		// Need to ensure we copy files from "b.Root/<import-path>" for the following command.
		err = ctx.CopyPackage(dest, src, root, vp.Path, ignoreFiles, vp.Tree, h, nil)
		if err != nil {
			fmt.Fprintf(ctx, "failed to copy package from %q to %q: %+v", src, dest, err)
		}
		checksum := h.Sum(nil)
		h.Reset()
		vp.ChecksumSHA1 = base64.StdEncoding.EncodeToString(checksum)
		updatedVendorFile = true
	}

	// Only write a vendor file if something changes.
	if updatedVendorFile {
		err = ctx.WriteVendorFile()
		if err != nil {
			return err
		}
	}

	// Return network errors here.
	if len(rem) > 0 {
		return rem
	}

	return nil
}
