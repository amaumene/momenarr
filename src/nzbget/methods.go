package nzbget

import (
	"context"
	"time"
)

// Version returns the NZBGet Version.
// https://nzbget.net/api/version
func (n *NZBGet) Version() (string, error) {
	return n.VersionContext(context.Background())
}

// VersionContext returns the NZBGet Version.
// https://nzbget.net/api/version
func (n *NZBGet) VersionContext(ctx context.Context) (string, error) {
	var output string
	err := n.GetInto(ctx, "version", &output)

	return output, err
}

// ListFiles returns the NZBGet Files for a download.
// https://nzbget.net/api/listfiles
// nzbID is the NZBID of the group to be returned. Use 0 for all file groups.
func (n *NZBGet) ListFiles(nzbID int64) (*File, error) {
	return n.ListFilesContext(context.Background(), nzbID)
}

// ListFilesContext returns the NZBGet Files for a download.
// https://nzbget.net/api/listfiles
// nzbID is the NZBID of the group to be returned. Use 0 for all file groups.
func (n *NZBGet) ListFilesContext(ctx context.Context, nzbID int64) (*File, error) {
	var output File
	err := n.GetInto(ctx, "listfiles", &output, 0, 0, nzbID)

	return &output, err
}

// Status returns the NZBGet Status.
// https://nzbget.net/api/status
func (n *NZBGet) Status() (*Status, error) {
	return n.StatusContext(context.Background())
}

// StatusContext returns the NZBGet Status.
// https://nzbget.net/api/status
func (n *NZBGet) StatusContext(ctx context.Context) (*Status, error) {
	var output Status
	err := n.GetInto(ctx, "status", &output)

	return &output, err
}

// History returns the NZBGet Download History.
// https://nzbget.net/api/history
func (n *NZBGet) History(hidden bool) ([]*History, error) {
	return n.HistoryContext(context.Background(), hidden)
}

// HistoryContext returns the NZBGet Download History.
// https://nzbget.net/api/history
func (n *NZBGet) HistoryContext(ctx context.Context, hidden bool) ([]*History, error) {
	var output []*History
	err := n.GetInto(ctx, "history", &output, hidden)

	return output, err
}

// ListGroups returns the NZBGet Download list.
// https://nzbget.net/api/listgroups
func (n *NZBGet) ListGroups() ([]*Group, error) {
	return n.ListGroupsContext(context.Background())
}

// ListGroupsContext returns the NZBGet Download list.
// https://nzbget.net/api/listgroups
func (n *NZBGet) ListGroupsContext(ctx context.Context) ([]*Group, error) {
	var output []*Group
	err := n.GetInto(ctx, "listgroups", &output, 0)

	return output, err
}

// Log returns the NZBGet Logs.
// NOTE: only one parameter - either startID or limit - can be specified. The other parameter must be 0.
// https://nzbget.net/api/log
func (n *NZBGet) Log(startID, limit int64) ([]*LogEntry, error) {
	return n.LogContext(context.Background(), startID, limit)
}

// LogContext returns the NZBGet Logs.
// NOTE: only one parameter - either startID or limit - can be specified. The other parameter must be 0.
// https://nzbget.net/api/log
func (n *NZBGet) LogContext(ctx context.Context, startID, limit int64) ([]*LogEntry, error) {
	var output []*LogEntry
	err := n.GetInto(ctx, "log", &output, startID, limit)

	return output, err
}

// LoadLog returns the NZBGet log for a specific download.
// NOTE: only one of either startID or limit - can be specified. The other parameter must be 0.
// https://nzbget.net/api/loadlog
func (n *NZBGet) LoadLog(nzbID, startID, limit int64) ([]*LogEntry, error) {
	return n.LoadLogContext(context.Background(), nzbID, startID, limit)
}

// LoadLogContext returns the NZBGet log for a specific download.
// NOTE: only one of either startID or limit - can be specified. The other parameter must be 0.
// https://nzbget.net/api/loadlog
func (n *NZBGet) LoadLogContext(ctx context.Context, nzbID, startID, limit int64) ([]*LogEntry, error) {
	var output []*LogEntry
	err := n.GetInto(ctx, "loadlog", &output, nzbID, startID, limit)

	return output, err
}

// Config returns the loaded and active NZBGet configuration parameters.
// https://nzbget.net/api/config
func (n *NZBGet) Config() ([]*Parameter, error) {
	return n.ConfigContext(context.Background())
}

// ConfigContext returns the loaded and active NZBGet configuration parameters.
// https://nzbget.net/api/config
func (n *NZBGet) ConfigContext(ctx context.Context) ([]*Parameter, error) {
	var output []*Parameter
	err := n.GetInto(ctx, "config", &output)

	return output, err
}

// LoadConfig returns the configuration from disk.
// https://nzbget.net/api/loadconfig
func (n *NZBGet) LoadConfig() ([]*Parameter, error) {
	return n.LoadConfigContext(context.Background())
}

// LoadConfigContext returns the configuration from disk.
// https://nzbget.net/api/loadconfig
func (n *NZBGet) LoadConfigContext(ctx context.Context) ([]*Parameter, error) {
	var output []*Parameter
	err := n.GetInto(ctx, "loadconfig", &output)

	return output, err
}

// SaveConfig writes new configuration parameters to disk.
// https://nzbget.net/api/saveconfig
func (n *NZBGet) SaveConfig(configs []*Parameter) (bool, error) {
	return n.SaveConfigContext(context.Background(), configs)
}

// SaveConfigContext writes new configuration parameters to disk.
// https://nzbget.net/api/saveconfig
func (n *NZBGet) SaveConfigContext(ctx context.Context, configs []*Parameter) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "saveconfig", &output, configs)

	return output, err
}

// Shutdown makes NZBGet exit.
// https://nzbget.net/api/shutdown
func (n *NZBGet) Shutdown() (bool, error) {
	return n.ShutdownContext(context.Background())
}

// ShutdownContext makes NZBGet exit.
// https://nzbget.net/api/shutdown
func (n *NZBGet) ShutdownContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "shutdown", &output)

	return output, err
}

// Reload makes NZBGet stop all activities and reinitialize.
// https://nzbget.net/api/reload
func (n *NZBGet) Reload() (bool, error) {
	return n.ReloadContext(context.Background())
}

// ReloadContext makes NZBGet stop all activities and reinitialize.
// https://nzbget.net/api/reload
func (n *NZBGet) ReloadContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "reload", &output)

	return output, err
}

// Rate sets download speed limit.
// https://nzbget.net/api/rate
func (n *NZBGet) Rate(limit int64) (bool, error) {
	return n.RateContext(context.Background(), limit)
}

// RateContext sets download speed limit.
// https://nzbget.net/api/rate
func (n *NZBGet) RateContext(ctx context.Context, limit int64) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "rate", &output, limit)

	return output, err
}

// PausePost pauses post processing.
// https://nzbget.net/api/pausepost
func (n *NZBGet) PausePost() (bool, error) {
	return n.PausePostContext(context.Background())
}

// PausePostContext pauses post processing.
// https://nzbget.net/api/pausepost
func (n *NZBGet) PausePostContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "pausepost", &output)

	return output, err
}

// ResumePost resumes post processing.
// https://nzbget.net/api/resumepost
func (n *NZBGet) ResumePost() (bool, error) {
	return n.ResumePostContext(context.Background())
}

// ResumePostContext resumes post processing.
// https://nzbget.net/api/resumepost
func (n *NZBGet) ResumePostContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "resumepost", &output)

	return output, err
}

// PauseDownload pauses downloads.
// https://nzbget.net/api/pausedownload
func (n *NZBGet) PauseDownload() (bool, error) {
	return n.PauseDownloadContext(context.Background())
}

// PauseDownloadContext pauses downloads.
// https://nzbget.net/api/pausedownload
func (n *NZBGet) PauseDownloadContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "pausedownload", &output)

	return output, err
}

// ResumeDownload resumes downloads.
// https://nzbget.net/api/resumedownload
func (n *NZBGet) ResumeDownload() (bool, error) {
	return n.ResumeDownloadContext(context.Background())
}

// ResumeDownloadContext resumes downloads.
// https://nzbget.net/api/resumedownload
func (n *NZBGet) ResumeDownloadContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "resumedownload", &output)

	return output, err
}

// PauseScan pauses scanning of directory with incoming nzb-files.
// https://nzbget.net/api/pausescan
func (n *NZBGet) PauseScan() (bool, error) {
	return n.PauseScanContext(context.Background())
}

// PauseScanContext pauses scanning of directory with incoming nzb-files.
// https://nzbget.net/api/pausescan
func (n *NZBGet) PauseScanContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "pausescan", &output)

	return output, err
}

// ResumeScan resumes scanning of directory with incoming nzb-files.
// https://nzbget.net/api/resumescan
func (n *NZBGet) ResumeScan() (bool, error) {
	return n.ResumeScanContext(context.Background())
}

// ResumeScanContext resumes scanning of directory with incoming nzb-files.
// https://nzbget.net/api/resumescan
func (n *NZBGet) ResumeScanContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "resumescan", &output)

	return output, err
}

// ScheduleResume schedules resuming of all activities after wait duration elapses.
// Wait duration is rounded to nearest second.
// https://nzbget.net/api/scheduleresume
func (n *NZBGet) ScheduleResume(wait time.Duration) (bool, error) {
	return n.ScheduleResumeContext(context.Background(), wait)
}

// ScheduleResumeContext schedules resuming of all activities after wait duration elapses.
// Wait duration is rounded to nearest second.
// https://nzbget.net/api/scheduleresume
func (n *NZBGet) ScheduleResumeContext(ctx context.Context, wait time.Duration) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "scheduleresume", &output, wait.Seconds())

	return output, err
}

// Scan requests rescanning of incoming directory for nzb-files.
// https://nzbget.net/api/scheduleresume
func (n *NZBGet) Scan() (bool, error) {
	return n.ScanContext(context.Background())
}

// ScanContext requests rescanning of incoming directory for nzb-files.
// https://nzbget.net/api/scheduleresume
func (n *NZBGet) ScanContext(ctx context.Context) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "scan", &output)

	return output, err
}

// WriteLog appends a log entry to th server's log and on-screen log buffer.
// https://nzbget.net/api/writelog
func (n *NZBGet) WriteLog(kind LogKind, text string) (bool, error) {
	return n.WriteLogContext(context.Background(), kind, text)
}

// WriteLogContext appends a log entry to th server's log and on-screen log buffer.
// https://nzbget.net/api/writelog
func (n *NZBGet) WriteLogContext(ctx context.Context, kind LogKind, text string) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "writelog", &output, kind, text)

	return output, err
}

// ConfigTemplates returns NZBGet configuration file template and
// also extracts configuration sections from all post-processing files.
// This information is for example used by web-interface to build settings
// page or page “Postprocess” in download details dialog.
// https://nzbget.net/api/configtemplates
func (n *NZBGet) ConfigTemplates(loadFromDisk bool) ([]*ConfigTemplate, error) {
	return n.ConfigTemplatesContext(context.Background(), loadFromDisk)
}

// ConfigTemplatesContext returns NZBGet configuration file template and
// also extracts configuration sections from all post-processing files.
// This information is for example used by web-interface to build settings
// page or page “Postprocess” in download details dialog.
// https://nzbget.net/api/configtemplates
func (n *NZBGet) ConfigTemplatesContext(ctx context.Context, loadFromDisk bool) ([]*ConfigTemplate, error) {
	var output []*ConfigTemplate
	err := n.GetInto(ctx, "configtemplates", &output, loadFromDisk)

	return output, err
}

// ServerVolumes returns download volume statistics per news-server.
// https://nzbget.net/api/servervolumes
// NOTE: The first record (serverid=0) are totals for all servers.
func (n *NZBGet) ServerVolumes() ([]*ServerVolume, error) {
	return n.ServerVolumesContext(context.Background())
}

// ServerVolumesContext returns download volume statistics per news-server.
// https://nzbget.net/api/servervolumes
// NOTE: The first record (serverid=0) are totals for all servers.
func (n *NZBGet) ServerVolumesContext(ctx context.Context) ([]*ServerVolume, error) {
	var output []*ServerVolume
	err := n.GetInto(ctx, "servervolumes", &output)

	return output, err
}

// ResetServerVolume resets download volume statistics for a specified news-server.
// https://nzbget.net/api/resetservervolume
func (n *NZBGet) ResetServerVolume(serverID int64, sounter string) (bool, error) {
	return n.ResetServerVolumeContext(context.Background(), serverID, sounter)
}

// ResetServerVolumeContext resets download volume statistics for a specified news-server.
// https://nzbget.net/api/resetservervolume
func (n *NZBGet) ResetServerVolumeContext(ctx context.Context, serverID int64, sounter string) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "resetservervolume", &output, serverID, sounter)

	return output, err
}

// AppendInput is the input data for the append method.
// See https://nzbget.net/api/append for more information about this data.
type AppendInput struct {
	Filename   string
	Content    string
	Category   string
	Priority   int64
	AddToTop   bool
	AddPaused  bool
	DupeKey    string // See: https://nzbget.net/rss#duplicate-keys
	DupeScore  int64  // See: https://nzbget.net/rss#duplicate-scores
	DupeMode   string // See: https://nzbget.net/rss#duplicate-modes
	Parameters []*Parameter
}

// Append adds a nzb-file or URL to the download queue.
// https://nzbget.net/api/append
func (n *NZBGet) Append(input *AppendInput) (int64, error) {
	return n.AppendContext(context.Background(), input)
}

// AppendContext adds a nzb-file or URL to the download queue.
// https://nzbget.net/api/append
func (n *NZBGet) AppendContext(ctx context.Context, input *AppendInput) (int64, error) {
	var output int64
	err := n.GetInto(ctx, "append", &output,
		input.Filename,
		input.Content,
		input.Category,
		input.Priority,
		input.AddToTop,
		input.AddPaused,
		input.DupeKey,
		input.DupeScore,
		input.DupeMode,
		ppparameters(input.Parameters),
	)

	return output, err
}

// ppparameters turns input Parameters into an RPC-compatible format.
func ppparameters(parameters []*Parameter) interface{} {
	output := make([][]string, len(parameters))
	for idx, param := range parameters {
		output[idx] = []string{param.Name, param.Value}
	}

	return output
}

// EditQueue edits items in download queue or in history.
// Read the official docs for how to issue commands, and which commands are available.
// https://nzbget.net/api/editqueue
func (n *NZBGet) EditQueue(command, parameter string, ids []int64) (bool, error) {
	return n.EditQueueContext(context.Background(), command, parameter, ids)
}

// EditQueueContext edits items in download queue or in history.
// Read the official docs for how to issue commands, and which commands are available.
// https://nzbget.net/api/editqueue
func (n *NZBGet) EditQueueContext(ctx context.Context, command, parameter string, ids []int64) (bool, error) {
	var output bool
	err := n.GetInto(ctx, "editqueue", &output, command, parameter, ids)

	return output, err
}
