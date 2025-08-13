package handlers

const mediaPageTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Momenarr - Media List</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #333;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            background-color: white;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th {
            background-color: #4CAF50;
            color: white;
            padding: 12px;
            text-align: left;
            position: sticky;
            top: 0;
        }
        td {
            padding: 10px;
            border-bottom: 1px solid #ddd;
        }
        tr:hover {
            background-color: #f5f5f5;
        }
        .status-on-disk {
            color: green;
            font-weight: bold;
        }
        .status-not-on-disk {
            color: orange;
            font-weight: bold;
        }
        .status-downloading {
            color: blue;
            font-weight: bold;
        }
        .type-movie {
            background-color: #e3f2fd;
        }
        .type-episode {
            background-color: #f3e5f5;
        }
        .stats {
            margin-bottom: 20px;
            padding: 15px;
            background-color: white;
            border-radius: 5px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .file-path {
            font-family: monospace;
            font-size: 0.9em;
            color: #555;
        }
        .powered-by {
            margin-top: 20px;
            text-align: center;
            color: #666;
        }
    </style>
</head>
<body>
    <h1>Momenarr Media Library</h1>
    
    <div class="stats">
        <h2>Statistics</h2>
        <p>Total Media: {{.Stats.Total}} | On Disk: {{.Stats.OnDisk}} | Not on Disk: {{.Stats.NotOnDisk}} | Downloading: {{.Stats.Downloading}}</p>
        <p>Movies: {{.Stats.Movies}} | Episodes: {{.Stats.Episodes}}</p>
    </div>

    <table>
        <thead>
            <tr>
                <th>Trakt ID</th>
                <th>Type</th>
                <th>Title</th>
                <th>Year</th>
                <th>Season/Episode</th>
                <th>Status</th>
                <th>File Path</th>
                <th>Created</th>
                <th>Updated</th>
            </tr>
        </thead>
        <tbody>
            {{range .Media}}
            <tr class="{{if .IsMovie}}type-movie{{else}}type-episode{{end}}">
                <td>{{.Trakt}}</td>
                <td>{{if .IsMovie}}Movie{{else}}Episode{{end}}</td>
                <td>{{.Title}}</td>
                <td>{{.Year}}</td>
                <td>{{if not .IsMovie}}S{{printf "%02d" .Season}}E{{printf "%02d" .Number}}{{else}}-{{end}}</td>
                <td>
                    {{if .OnDisk}}
                        <span class="status-on-disk">On Disk</span>
                    {{else if .IsDownloading}}
                        <span class="status-downloading">Downloading</span>
                    {{else}}
                        <span class="status-not-on-disk">Not on Disk</span>
                    {{end}}
                </td>
                <td class="file-path">{{if .File}}{{.File}}{{else}}-{{end}}</td>
                <td>{{.CreatedAt.Format "2006-01-02 15:04"}}</td>
                <td>{{.UpdatedAt.Format "2006-01-02 15:04"}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    
    <div class="powered-by">
        <p>Powered by Momenarr with AllDebrid</p>
    </div>
</body>
</html>
`