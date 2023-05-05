import * as React from 'react';
import { useState, useEffect } from 'react';
import http from '../http';
import AppBar from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import Container from '@mui/material/Container';
import { parseJSON , formatDistanceToNow, formatDuration, intervalToDuration, addMilliseconds } from'date-fns';
import { Button } from '@mui/material';

export default function ResponsiveAppBar() {
  const [running, setrunning] = useState(false)
  const [timeDiff, settimeDiff] = useState("")
  const [schedule, setschedule] = useState("")

  function fetchData() {
    http.get('/api/stats/current')
    .then(data => {
      const timeDiffMs = data.timeDiff / 1000_000;
      const now = new Date();
      const duration = intervalToDuration({ start: now, end: addMilliseconds(now, timeDiffMs) });
      // Add millisecond precision
      const millisecondsToAdd = Math.round(timeDiffMs % 1000) / 1000;
      duration.seconds += millisecondsToAdd;
      settimeDiff(formatDuration(duration));

      setrunning(data.running);
    });
    http.get('/api/schedule').then((data) => {
      const nextRun = data ? parseJSON(data) : new Date();
      setschedule(formatDistanceToNow(nextRun, { addSuffix: true }));
    });
  }

  function runCheckrr() {
    http.post("/api/run", {});
  }

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
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
              {running ? 'Running' : `Waiting for next run ${schedule && `(${schedule})`}`}
            </Typography>
          </Box>
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
