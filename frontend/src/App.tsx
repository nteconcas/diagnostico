import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './auth'
import { Layout } from './components/Layout'
import { Painel } from './pages/Painel'
import { Login } from './pages/Login'
import { Escolas } from './pages/Escolas'
import { Usuarios } from './pages/Usuarios'
import { Formulario } from './pages/Formulario'
import { Envio } from './pages/Envio'

function Protegido({
  children,
  admin = false,
  preenchimento = false,
}: {
  children: React.ReactNode
  admin?: boolean
  preenchimento?: boolean
}) {
  const { usuario, carregando } = useAuth()
  if (carregando) {
    return (
      <div className="carregando-pagina">
        <div className="girando" /> Carregando…
      </div>
    )
  }
  if (!usuario) return <Navigate to="/login" replace />
  if (admin && usuario.papel !== 'admin') return <Navigate to="/" replace />
  if (preenchimento && usuario.papel === 'admin') return <Navigate to="/" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        element={
          <Protegido>
            <Layout />
          </Protegido>
        }
      >
        <Route path="/" element={<Painel />} />
        <Route
          path="/formulario/:id"
          element={
            <Protegido preenchimento>
              <Formulario />
            </Protegido>
          }
        />
        <Route path="/envios/:id" element={<Envio />} />
        <Route
          path="/escolas"
          element={
            <Protegido admin>
              <Escolas />
            </Protegido>
          }
        />
        <Route
          path="/usuarios"
          element={
            <Protegido admin>
              <Usuarios />
            </Protegido>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}
