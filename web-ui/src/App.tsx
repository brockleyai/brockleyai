import { useAppStore } from "./store";
import ConnectionPage from "./pages/ConnectionPage";
import GraphListPage from "./pages/GraphListPage";
import GraphEditorPage from "./pages/GraphEditorPage";
import ExecutionPage from "./pages/ExecutionPage";
import APIToolListPage from "./pages/APIToolListPage";
import APIToolEditorPage from "./pages/APIToolEditorPage";
import Layout from "./components/Layout";
import { ToastProvider } from "./components/Toast";

export default function App() {
  const isConnected = useAppStore((s) => s.isConnected);
  const currentPage = useAppStore((s) => s.currentPage);

  if (!isConnected) {
    return <ConnectionPage />;
  }

  const pageContent = (() => {
    switch (currentPage) {
      case "graph-editor":
        return <GraphEditorPage />;
      case "execution":
        return <ExecutionPage />;
      case "api-tools":
        return <APIToolListPage />;
      case "api-tool-editor":
        return <APIToolEditorPage />;
      case "graphs":
      default:
        return <GraphListPage />;
    }
  })();

  return (
    <ToastProvider>
      <Layout>{pageContent}</Layout>
    </ToastProvider>
  );
}
