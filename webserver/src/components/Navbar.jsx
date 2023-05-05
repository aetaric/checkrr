import * as React from 'react';
import { useState, useEffect } from 'react';
import http from '../http';
import AppBar from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import Container from '@mui/material/Container';
import convertHrtime from 'convert-hrtime';
import { parseJSON , formatDistanceToNow, formatDuration, intervalToDuration, addMilliseconds } from'date-fns';
import { Button } from '@mui/material';

export default function ResponsiveAppBar() {
  const [running, setrunning] = useState(false)
  const [timeDiff, settimeDiff] = useState("")
  const [schedule, setschedule] = useState("")

  function fetchData() {
    http.get('/api/stats/current')
    .then(data => {
      const hrTime = convertHrtime(data.timeDiff);
      const now = new Date();
      const interval = intervalToDuration({ start: now, end: addMilliseconds(now, hrTime.milliseconds) });
      // Add millisecond precision
      const millisecondsToAdd = Math.round(hrTime.milliseconds % 1000) / 1000;
      const formattedDuration = formatDuration({ ...interval, seconds: interval.seconds + millisecondsToAdd });
      settimeDiff(formattedDuration);

      setrunning(data.running);
    });
    http.get('/api/schedule').then((data) => {
      const nextRun = data ? parseJSON(data) : new Date();
      setschedule(formatDistanceToNow(nextRun, { addSuffix: true }));
    });
    setTimeout(() => {fetchData()},10000)
  }

  function runCheckrr() {
    http.post("/api/run", {});
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
          {schedule && <Box sx={{ flexGrow: 0 }}>
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
              Next Run: {schedule}
            </Typography>
          </Box>}
          {timeDiff && <Box sx={{ flexGrow: 0 }}>
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
              Last Run: {timeDiff}
            </Typography>
          </Box>}
        </Toolbar>
      </Container>
    </AppBar>
  );
}
