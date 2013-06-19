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
  "time"
  "fmt"
  "runtime"
  "strings"
  "strconv"
)

func Log(format string, args ...interface{}) {
  if (*optVerbose == false) {
    return
  }

  now := time.Now()
  year, month, day := now.Date()
  hour, minute, second := now.Clock()
  time_str := fmt.Sprintf("[GOZD][%d-%d-%d %d:%d:%d]", year, month, day, hour, minute, second)
  
  var nameStr string
  pidStr := fmt.Sprintf("[%d]", runningPID)
  pc, _, _, _ := runtime.Caller(1)
  name := runtime.FuncForPC(pc).Name()
  names := strings.Split(name, ".")
  if len(names) > 0 {
    nameStr = names[len(names)-1]
  }
  callerStr := "[" + nameStr + "] "
  fmt.Printf(time_str + pidStr + callerStr + format + "\n", args...)
}

func LogErr(format string, args ...interface{}) {
  now := time.Now()
  year, month, day := now.Date()
  hour, minute, second := now.Clock()
  time_str := fmt.Sprintf("[GOZDERR][%d-%d-%d %d:%d:%d]", year, month, day, hour, minute, second)
  
  var fileStr, nameStr string
  pidStr := fmt.Sprintf("[%d]", runningPID)
  pc, _, _, _ := runtime.Caller(1)
  file, line := runtime.FuncForPC(pc).FileLine(pc)
  files := strings.Split(file, "/")
  if len(files) > 0 {
    fileStr = files[len(files)-1]
  }
  name := runtime.FuncForPC(pc).Name()
  names := strings.Split(name, ".")
  if len(names) > 0 {
    nameStr = names[len(names)-1]
  }
  callerStr := "[" + nameStr + " " + fileStr + ":" + strconv.Itoa(line) + "] "
  fmt.Printf(time_str + pidStr + callerStr + format + "\n", args...)
}
