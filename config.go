/*
 * Copyright (c) 2013, PinIdea Co. Ltd.
 * Tomasen <tomasen@gmail.com> & Reck Hou <reckhou@gmail.com>
 * All rights reserved.
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 *
 *     * Redistributions of source code must retain the above copyright
 *       notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above copyright
 *       notice, this list of conditions and the following disclaimer in the
 *       documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE REGENTS AND CONTRIBUTORS "AS IS" AND ANY
 * EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL THE COMPANY AND CONTRIBUTORS BE LIABLE FOR ANY
 * DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
 * LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
 * ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
 * SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

/* The idea is came from nginx and this post: http://blog.nella.org/?p=879
 * and code here: http://code.google.com/p/jra-go/source/browse/#hg%2Fcmd%2Fupgradable
 */

package gozd

import (
  "flag"
  "path/filepath"
  "strings"
  "io"
  "io/ioutil"
  "crypto/sha1"
  "syscall"
  "os"
  "strconv"
  "fmt"
)

// configurations
var (
  optSendSignal = flag.String("s", "", "Send signal to old process: <start, stop, quit, reopen, reload>.")
  optConfigFile = flag.String("c", "", "Set configuration file path." )
  optRunForeground = flag.Bool("f", false, "Running in foreground for debug.")
  optVerbose = flag.Bool("v", false, "Show GOZD log.")
  optHelp = flag.Bool("h", false, "This help")
  optGroups = make(map[string]*configGroup)
  openedFDs = make(map[string]*openedFD) // key = group name, this ONLY records FDs opened by old process, should be empty if using "-s start"
  gozdPrefix = "gozerodown" // used for SHA1 hash, change it with different daemons
)

func parseConfigFile(filePath string) bool {
  path, _ := filepath.Abs(filePath)
  configString, err := readStringFromFile(path)
  if err != nil {
    LogErr(err.Error())
    return false
  }

  newGroup := new(configGroup)
  splitLines := strings.Split(configString, "\n")
  groupName := ""
  for idx := 0; idx < len(splitLines); idx++ {
    param := extractParam(splitLines[idx])
    if param == "" {
      continue
    }
    
    if strings.Contains(splitLines[idx], "mode") {
      if groupName != "" {
        newGroup.mode = param
      }
    } else if strings.Contains(splitLines[idx], "listen") {
      if groupName != "" {
        newGroup.listen = param
      }
    } else {
      groupName = param
      optGroups[groupName] = newGroup
    }

    if groupName != "" && newGroup.mode != "" && newGroup.listen != "" {
      newGroup = new(configGroup)
      groupName = ""
    }
  }
  return true
}

// extract parameter between"[]"
func extractParam(raw string) string {
  if strings.Contains(raw, "#") {
    return ""
  }

  idxBegin := strings.Index(raw, "[")
  idxEnd := strings.Index(raw, "]")
  if idxBegin < 0 || idxEnd < 0 {
    return ""
  } else {
    resultByte := []byte(raw)
    return string(resultByte[idxBegin+1:idxEnd])
  }
}

func readStringFromFile(filepath string) (string, error) {
  contents, err := ioutil.ReadFile(filepath)
  Log("readStringFromFile:")
  Log(string(contents))
  return string(contents), err
}

func writeStringToFile(filepath string, contents string) error {
  return ioutil.WriteFile(filepath, []byte(contents), os.ModeAppend)
}

func truncateFile(filePath string) error {
  f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0777)
  if err != nil {
    LogErr(err.Error())
    return err
  }
  err = f.Truncate(0)
  if err != nil {
    LogErr(err.Error())
    return err
  }

  return err
}

func appendFile(filePath string, contents string) error {
  f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0777)
  if err != nil {
    return err
  }
  
  var n int
  n, err = f.Write([]byte(contents))
  f.Sync()
  f.Close()

  if err == nil && n < len(contents) {
    err = io.ErrShortWrite
  }
  
  return err
}

// Get running process info(pid, fd, etc...)
func getRunningInfoByConf(confPath string, prefix string) ([]string, error) {
  infoFilepath := getRunningInfoPathByConf(confPath, prefix)

  infoString, err := readStringFromFile(infoFilepath)
  Log("Info Filepath: " + infoFilepath)
  Log("Info string: ["+ infoString + "]")
  if (err != nil) {
    return []string{}, err
  }
  result := strings.Split(infoString, "|")
  return result, err
}

func resetRunningInfoByConf(confPath string, prefix string) {
  infoFilepath := getRunningInfoPathByConf(confPath, prefix)
  truncateFile(infoFilepath) // ignore any errors
  
  str := strconv.Itoa(os.Getpid())
  str += "|"
  err := appendFile(infoFilepath, str)
  if err != nil {
    LogErr(err.Error())
    return
  }
}

func getRunningInfoPathByConf(confPath string, prefix string) string {
  confPathAbs, err := filepath.Abs(confPath)
  if err != nil {
    LogErr(err.Error())
    return ""
  }

  hashSha1 := sha1.New()
  io.WriteString(hashSha1, confPathAbs)
  workPath, _ := filepath.Abs("")
  infoFilepath := filepath.Join(workPath, "/tmp/", fmt.Sprintf("%v_%x.gozd", prefix, hashSha1.Sum(nil)))
  syscall.Mkdir(workPath+"/tmp/", 0777)
  if err != nil && strings.Contains(err.Error(), "file exists") { // this is a "hack" solution
    LogErr(err.Error())
  }
  Log("confpath: " + confPathAbs)
  Log("Info file path: " + infoFilepath)
  return infoFilepath
}

