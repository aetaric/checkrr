import * as React from 'react';
import { useState, useEffect } from 'react';
import Typography from '@mui/material/Typography';
import { Paper } from "@mui/material";
import axios from 'axios';
import Grid from '@mui/material/Grid';
import { Chart as ChartJS, ArcElement, Tooltip, Legend, CategoryScale, LinearScale, PointElement, LineElement, Title } from 'chart.js';
import { Pie, Line } from 'react-chartjs-2';

export default function Stats() {
  const [piedata, setpiedata] = useState({labels: [],datasets: []})
  const [linedata, setlinedata] = useState({labels: [],datasets: []})
  const [colors, setColors] = useState([randomRGB(false),randomRGB(false),randomRGB(false),randomRGB(false),
    randomRGB(false),randomRGB(false),randomRGB(false),randomRGB(false),randomRGB(false),randomRGB(false),randomRGB(false)])

  ChartJS.register(ArcElement, Tooltip, Legend, CategoryScale, LinearScale, PointElement, LineElement, Title);

  function fetchData() {
    axios.get('/api/stats/current')
    .then(res => {
        let stats = res.data
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
                }
            ],
        }
        setpiedata(piedata)
    })
    axios.get('/api/stats/historical')
    .then(res => {
      let data = res.data
      // Fix the data so it's ready for chart.js
      let sortedData = { sonarrSubmissions: [], radarrSubmissions: [], lidarrSubmissions: [], filesChecked: [], hashMatches: [],
          hashMismatches: [], videoFiles: [], audioFiles: [], unknownFileCount: [], unknownFilesDeleted: [], nonVideo: [] }
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
                  case "unknownFilesDeleted":
                      sortedData.unknownFilesDeleted.push(d[k])
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
          let dataset = {label: k, data: sortedData[k], backgroundColor: colors[i]}
          datasets.push(dataset)
          i++
      }
      let linedata = { labels: label, datasets: datasets }
      setlinedata(linedata)
    })
    setTimeout(() => {fetchData()},10000)
  }

  function randomRGB(border = false) {
    let a = 0.0
    if (border) {
        a = 1.0
    } else {
        a = 0.8
    }
    let o = Math.round, r = Math.random, s = 255;
    let red = o(r()*s)
    let green = o(r()*s)
    let blue = o(r()*s)
    return `rgba(${red}, ${green}, ${blue}, ${a})`
  }

  useEffect(() => {
    fetchData()
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