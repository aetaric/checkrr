import * as React from 'react';
import { useState, useEffect } from 'react';
import axios from 'axios';
import AppBar from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import Container from '@mui/material/Container';
import convertHrtime from 'convert-hrtime';
import moment from 'moment';
import { Button } from '@mui/material';

export default function ResponsiveAppBar() {
  const [running, setrunning] = useState(false)
  const [timeDiff, settimeDiff] = useState({})
  const [schedule, setschedule] = useState("")

  function fetchData() {
    axios.get('/api/stats/current')
    .then(res => {
      let data = res.data
      if (data.timeDiff !== 0) { 
        settimeDiff(prettyPrintTime(convertHrtime(data.timeDiff)))
      } else {
        settimeDiff("0ms")
      }
      setrunning(data.running)
    })
    axios.get('/api/schedule')
    .then(res => {
      let data = res.data
      if (data != null) {
        setschedule(data)
      } else {
        setschedule(new Date().toISOString())
      }
    })
    setTimeout(() => {fetchData()},10000)
  }

  function prettyPrintTime(data) {
    let msec = data.milliseconds
    var hh = Math.floor(msec / 1000 / 60 / 60);
    msec -= hh * 1000 * 60 * 60;
    var mm = Math.floor(msec / 1000 / 60);
    msec -= mm * 1000 * 60;
    var ss = Math.floor(msec / 1000);
    msec -= ss * 1000;
    var ms = Math.round(msec)
    if (hh !== 0) {
      return `${hh}h ${mm}m ${ss}s ${ms}ms`
    } else if (mm !== 0) {
      return `${mm}m ${ss}s ${ms}ms`
    } else if (ss !== 0) {
      return `${ss}s ${ms}ms`
    } else {
      return `${ms}ms`
    }
  }

  function runCheckrr() {
    axios.post('/api/run', {}).then(res => {
      return
    })
  }

  useEffect(() => {
    fetchData()
    // eslint-disable-next-line
  },[])

  return (
    <AppBar position="static">
      <Container>
        <Toolbar disableGutters>
          <Typography
            variant="h6"
            noWrap
            component="a"
            href="/"
            sx={{
              mr: 2,
              display: { xs: 'none', md: 'flex' },
              fontFamily: 'monospace',
              fontWeight: 700,
              letterSpacing: '.3rem',
              color: 'inherit',
              textDecoration: 'none',
            }}
          >
            checkrr
          </Typography>
          <Box sx={{ flexGrow: 0}}>
            <Button disabled={running} variant="contained" size="small" onClick={ () => {runCheckrr(); setrunning(true)}}>Run Now</Button>&nbsp;
          </Box>
          <Box sx={{ flexGrow: 1, display: { xs: "none", md: "flex" } }}>
            <Typography
              variant="h8"
              noWrap
              component="a"
              href="/"
              sx={{
                mr: 2,
                display: { xs: 'none', md: 'flex' },
                fontFamily: 'monospace',
                fontWeight: 300,
                letterSpacing: '.01rem',
                color: 'inherit',
                textDecoration: 'none',
              }}
            >
              {running ? "Running" : "Waiting for next run"}
            </Typography>
          </Box>
          <Box sx={{ flexGrow: 0 }}>
            <Typography
              variant="h8"
              noWrap
              component="a"
              href="/"
              sx={{
                mr: 2,
                display: { xs: 'none', md: 'flex' },
                fontFamily: 'monospace',
                fontWeight: 300,
                letterSpacing: '.01rem',
                color: 'inherit',
                textDecoration: 'none',
              }}
            >
              {"Next Run: " + moment(schedule).fromNow()}
            </Typography>
          </Box>
          <Box sx={{ flexGrow: 0 }}>
            <Typography
              variant="h8"
              noWrap
              component="a"
              href="/"
              sx={{
                mr: 2,
                display: { xs: 'none', md: 'flex' },
                fontFamily: 'monospace',
                fontWeight: 300,
                letterSpacing: '.01rem',
                color: 'inherit',
                textDecoration: 'none',
              }}
            >
              {"Last Run: " + timeDiff}
            </Typography>
          </Box>
        </Toolbar>
      </Container>
    </AppBar>
  );
}