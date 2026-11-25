import { createBrowserRouter, RouterProvider } from "react-router";
import { Layout } from "./app/Layout";
import { AdminPage } from "./pages/Admin";
import { AdminLivePage } from "./pages/AdminLive";
import { AdminRunPage } from "./pages/AdminRun";
import { AdminRunsPage } from "./pages/AdminRuns";
import { MyProductsPage } from "./pages/MyProducts";
import { ProductPage } from "./pages/Product";
import { SearchPage } from "./pages/Search";
import { SignInPage } from "./pages/SignIn";

const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { path: "/", element: <SearchPage /> },
      { path: "/p/:productId", element: <ProductPage /> },
      { path: "/signin", element: <SignInPage /> },
      { path: "/me", element: <MyProductsPage /> },
      { path: "/admin", element: <AdminPage /> },
      { path: "/admin/live", element: <AdminLivePage /> },
      { path: "/admin/runs", element: <AdminRunsPage /> },
      { path: "/admin/runs/:runId", element: <AdminRunPage /> },
    ],
  },
]);

export function App() {
  return <RouterProvider router={router} />;
}
