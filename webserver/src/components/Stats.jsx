import * as React from 'react';
import { useState, useEffect } from 'react';
import Typography from '@mui/material/Typography';
import { Paper } from "@mui/material";
import http from '../http';
import Grid from '@mui/material/Grid';
import { Chart as ChartJS, ArcElement, Tooltip, Legend, CategoryScale, LinearScale, PointElement, LineElement, Title } from 'chart.js';
import { Pie, Line } from 'react-chartjs-2';

export default function Stats() {
  const [piedata, setpiedata] = useState({labels: [],datasets: []})
  const [linedata, setlinedata] = useState({labels: [],datasets: []})
  const [colors] = useState(["rgba(150, 11, 143, 0.5)","rgba(80, 137, 25, 0.5)","rgba(139, 43, 254, 0.5)","rgba(250, 39, 49, 0.5)","rgba(37, 99, 151, 0.5)",
  "rgba(188, 33, 3, 0.5)","rgba(38, 46, 252, 0.5)","rgba(248, 185, 75, 0.5)","rgba(251, 133, 55, 0.5)","rgba(139, 227, 251, 0.5)","rgba(94, 166, 191, 0.5)"])
  const [borderColors] = useState(["rgb(150, 11, 143)","rgb(80, 137, 25)","rgb(139, 43, 254)","rgb(250, 39, 49)","rgb(37, 99, 151)",
  "rgb(188, 33, 3)","rgb(38, 46, 252)","rgb(248, 185, 75)","rgb(251, 133, 55)","rgb(139, 227, 251)","rgb(94, 166, 191)"])

  ChartJS.register(ArcElement, Tooltip, Legend, CategoryScale, LinearScale, PointElement, LineElement, Title);

  function fetchData() {
    http.get('./api/stats/current')
    .then(stats => {
        let labels = []
        let data = []

        for (var k in stats) {
            if (k === "running") {
                continue
            }else if (k === "timeDiff") {
              continue
            } else {
                labels.push(k)
                data.push(stats[k])
            }
        }
        const piedata = {
            labels: labels,
            datasets: [
                {
                    label: "# of files",
                    data: data,
                    borderWidth: 1,
                    backgroundColor: colors,
                    borderColor: borderColors
                }
            ],
        }
        setpiedata(piedata)
    })
    http.get('./api/stats/historical')
    .then(data => {
      // Fix the data so it's ready for chart.js
      let sortedData = { sonarrSubmissions: [], radarrSubmissions: [], lidarrSubmissions: [], filesChecked: [], hashMatches: [],
          hashMismatches: [], videoFiles: [], audioFiles: [], unknownFileCount: [], nonVideo: [] }
      let label = []
      for (var obj in data) {
          let d = data[obj].Data
          label.push(data[obj].Timestamp)
          for (var k in d) {
              switch(k) {
                  case "sonarrSubmission":
                      sortedData.sonarrSubmissions.push(d[k])
                      break;
                  case "radarrSubmissions":
                      sortedData.radarrSubmissions.push(d[k])
                      break;
                  case "lidarSubmissions":
                      sortedData.lidarrSubmissions.push(d[k])
                      break;
                  case "filesChecked":
                      sortedData.filesChecked.push(d[k])
                      break;
                  case "hashMatches":
                      sortedData.hashMatches.push(d[k])
                      break;
                  case "hashMismatches":
                      sortedData.hashMismatches.push(d[k])
                      break;
                  case "videoFiles":
                      sortedData.videoFiles.push(d[k])
                      break;
                  case "audioFiles":
                      sortedData.audioFiles.push(d[k])
                      break;
                  case "unknownFileCount":
                      sortedData.unknownFileCount.push(d[k])
                      break;
                  case "nonVideo":
                      sortedData.nonVideo.push(d[k])
                      break;
                  default:
                      break;
              }
          }
      }
      // loop over the data to inject it into chart.js
      let datasets = []
      let i = 0
      // eslint-disable-next-line
      for (var k in sortedData) {
          let dataset = {label: k, data: sortedData[k], backgroundColor: colors[i], borderColor: borderColors[i] }
          datasets.push(dataset)
          i++
      }
      let linedata = { labels: label, datasets: datasets }
      setlinedata(linedata)
    })
  }
  
  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
    // eslint-disable-next-line
  },[])

  const pieOptions = {
    plugins: {
      title: {
        display: true,
        text: 'Last Run',
      },
    },
  };
  const lineOptions = {
    plugins: {
      title: {
        display: true,
        text: 'Historical Stats',
      },
    },
  };
  return (
    <Paper elevation={3}>
      <Typography
        variant="h6"
        noWrap
        component="a"
        href="/"
        style={{paddingTop: 20, paddingBottom: 5, paddingLeft: 20}}
        sx={{
          mr: 2,
          display: { xs: 'none', md: 'flex' },
          fontFamily: 'monospace',
          fontWeight: 700,
          letterSpacing: '.05rem',
          color: 'inherit',
          textDecoration: 'none',
        }}
      >
        Stats
      </Typography>
      <br/>
      <Grid container spacing={2}>
        <Grid item xs={4}>
            <Pie data={piedata} options={pieOptions}/>
        </Grid>
        <Grid item xs={8}>
            <Line data={linedata} options={lineOptions}/>
        </Grid>
      </Grid>
      <br/>
    </Paper>
  );
}
