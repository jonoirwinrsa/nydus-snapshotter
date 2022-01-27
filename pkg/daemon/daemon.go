/*
 * Copyright (c) 2020. Ant Group. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containerd/nydus-snapshotter/config"
	"github.com/containerd/nydus-snapshotter/pkg/nydussdk"
	"github.com/containerd/nydus-snapshotter/pkg/nydussdk/model"
	"github.com/pkg/errors"
)

const (
	APISocketFileName   = "api.sock"
	SharedNydusDaemonID = "shared_daemon"
)

type NewDaemonOpt func(d *Daemon) error

type Daemon struct {
	ID               string
	SnapshotID       string
	ConfigDir        string
	SocketDir        string
	LogDir           string
	LogLevel         string
	LogToStdout      bool
	SnapshotDir      string
	Pid              int
	ImageID          string
	DaemonMode       string
	apiSock          *string
	RootMountPoint   *string
	CustomMountPoint *string
	nydusdThreadNum  int
}

func (d *Daemon) SharedMountPoint() string {
	return filepath.Join(*d.RootMountPoint, d.SnapshotID, "fs")
}

func (d *Daemon) MountPoint() string {
	if d.RootMountPoint != nil {
		return filepath.Join("/", d.SnapshotID, "fs")
	}
	if d.CustomMountPoint != nil {
		return *d.CustomMountPoint
	}
	return filepath.Join(d.SnapshotDir, d.SnapshotID, "fs")
}

func (d *Daemon) OldMountPoint() string {
	return filepath.Join(d.SnapshotDir, d.SnapshotID, "fs")
}

func (d *Daemon) BootstrapFile() (string, error) {
	return GetBootstrapFile(d.SnapshotDir, d.SnapshotID)
}

func (d *Daemon) ConfigFile() string {
	return filepath.Join(d.ConfigDir, "config.json")
}

// NydusdThreadNum returns `nydusd-thread-num` for nydusd if set,
// otherwise will return the number of CPUs as default.
func (d *Daemon) NydusdThreadNum() string {
	if d.nydusdThreadNum > 0 {
		return strconv.Itoa(d.nydusdThreadNum)
	}
	// if nydusd-thread-num is not set, return empty string
	// to let manager don't set thread-num option.
	return ""
}

func (d *Daemon) APISock() string {
	if d.apiSock != nil {
		return *d.apiSock
	}
	return filepath.Join(d.SocketDir, APISocketFileName)
}

func (d *Daemon) LogFile() string {
	return filepath.Join(d.LogDir, "stderr.log")
}

func (d *Daemon) CheckStatus() (model.DaemonInfo, error) {
	client, err := nydussdk.NewNydusClient(d.APISock())
	if err != nil {
		return model.DaemonInfo{}, errors.Wrap(err, "failed to check status, client has not been initialized")
	}
	return client.CheckStatus()
}

func (d *Daemon) SharedMount() error {
	client, err := nydussdk.NewNydusClient(d.APISock())
	if err != nil {
		return errors.Wrap(err, "failed to mount")
	}
	bootstrap, err := d.BootstrapFile()
	if err != nil {
		return err
	}
	return client.SharedMount(d.MountPoint(), bootstrap, d.ConfigFile())
}

func (d *Daemon) SharedUmount() error {
	client, err := nydussdk.NewNydusClient(d.APISock())
	if err != nil {
		return errors.Wrap(err, "failed to mount")
	}
	return client.Umount(d.MountPoint())
}

func (d *Daemon) IsMultipleDaemon() bool {
	return d.DaemonMode == config.DaemonModeMultiple
}

func (d *Daemon) IsSharedDaemon() bool {
	return d.DaemonMode == config.DaemonModeShared
}

func (d *Daemon) IsPrefetchDaemon() bool {
	return d.DaemonMode == config.DaemonModePrefetch
}

func NewDaemon(opt ...NewDaemonOpt) (*Daemon, error) {
	d := &Daemon{Pid: 0}
	d.ID = newID()
	d.DaemonMode = config.DefaultDaemonMode
	for _, o := range opt {
		err := o(d)
		if err != nil {
			return nil, err
		}
	}
	return d, nil
}

func GetBootstrapFile(dir, id string) (string, error) {
	// the meta file is stored to <snapshotid>/image/image.boot
	bootstrap := filepath.Join(dir, id, "fs", "image", "image.boot")
	_, err := os.Stat(bootstrap)
	if err == nil {
		return bootstrap, nil
	}
	if os.IsNotExist(err) {
		// for backward compatibility check meta file from legacy location
		bootstrap = filepath.Join(dir, id, "fs", "image.boot")
		_, err = os.Stat(bootstrap)
		if err == nil {
			return bootstrap, nil
		}
	}
	return "", errors.Wrap(err, fmt.Sprintf("failed to find bootstrap file for ID %s", id))
}
