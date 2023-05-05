import { Container } from '@mui/system';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import ResponsiveAppBar from './components/Navbar';
import Stats from './components/Stats';
import DataTable from './components/Table';

const darkTheme = createTheme({
  palette: {
    mode: 'dark',
  },
});

function App() {
  return (
    <ThemeProvider theme={darkTheme}>
      <CssBaseline />
      <ResponsiveAppBar></ResponsiveAppBar>
      <Container maxWidth="xl">
        <Container maxWidth="xl">
          <br />
          <Stats />
          <br />
          <DataTable />
        </Container>
      </Container>
    </ThemeProvider>
  );
}

export default App;
