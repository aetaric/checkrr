import { Container } from "@mui/system";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import ResponsiveAppBar from "./components/Navbar"
import Stats from "./components/Stats"
import DataTable from "./components/Table";

const darkTheme = createTheme({
  palette: {
    mode: 'dark',
  },
});

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/"
          element={
            <ThemeProvider theme={darkTheme}>
              <CssBaseline />
              <ResponsiveAppBar></ResponsiveAppBar>
              <Container fixed>
                <Container maxWidth="xl">
                  <br/>
                  <Stats/>
                  <br/>
                  <DataTable/>
                </Container>
              </Container>
            </ThemeProvider>
          }
        />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
