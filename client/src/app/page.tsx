"use client";
import axios from "axios";
import { useState, useEffect } from "react";
import Markdown from "react-markdown";

// Custom CSS for hiding scrollbars while allowing scrolling
const scrollbarHideStyles = `
  .scrollbar-hide {
    -ms-overflow-style: none;  /* IE and Edge */
    scrollbar-width: none;  /* Firefox */
  }
  .scrollbar-hide::-webkit-scrollbar {
    display: none; /* Chrome, Safari, Opera */
  }
`;

const API_BASE_URL = "http://localhost:8080";
const API_VERSION = "/api/v1";

type Embedding = {
  id: number;
  file_path: string;
  text: string;
  embedding: Array<number>;
  is_batch?: boolean;
  batch_id?: string;
  batch_paths?: Array<string>;
};

type TaskStatus = {
  task_id: string;
  status: string;
  result?: any;
};

export default function Home() {
  const [files, setFiles] = useState<File[]>([]);
  const [query, setQuery] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(false);
  const [uploadLoading, setUploadLoading] = useState<boolean>(false);
  const [processingTasks, setProcessingTasks] = useState<
    Map<string, TaskStatus>
  >(new Map());
  const [results, setResults] = useState<Array<Embedding>>([]);
  const [notification, setNotification] = useState<{
    message: string;
    type: "success" | "error" | "info";
  } | null>(null);
  const [batchAnalyze, setBatchAnalyze] = useState<boolean>(false);
  const [selectedBatchImage, setSelectedBatchImage] = useState<string>("");

  useEffect(() => {
    const taskIds = Array.from(processingTasks.keys()).filter(
      (id) =>
        processingTasks.get(id)?.status !== "completed" &&
        processingTasks.get(id)?.status !== "failed",
    );

    if (taskIds.length === 0) return;

    const checkInterval = setInterval(async () => {
      for (const taskId of taskIds) {
        try {
          const response = await axios.get(
            `${API_BASE_URL}${API_VERSION}/tasks/${taskId}`,
          );
          const taskStatus = response.data;

          setProcessingTasks((prev) => {
            const updated = new Map(prev);
            updated.set(taskId, taskStatus);
            return updated;
          });

          if (taskStatus.status === "completed") {
            showNotification("Image processing completed!", "success");

            if (query) {
              handleSearch();
            }
          } else if (taskStatus.status === "failed") {
            showNotification("Image processing failed", "error");
          }
        } catch (error) {
          console.error(`Error checking task ${taskId} status:`, error);
        }
      }
    }, 2000);

    return () => clearInterval(checkInterval);
  }, [processingTasks, query]);

  const showNotification = (
    message: string,
    type: "success" | "error" | "info",
  ) => {
    setNotification({ message, type });
    setTimeout(() => setNotification(null), 3000);
  };

  const uploadImage = async () => {
    if (files.length === 0) {
      showNotification("Please select at least one image", "error");
      return;
    }

    if (files.length > 5) {
      showNotification("Maximum 5 images allowed", "error");
      return;
    }

    setUploadLoading(true);
    try {
      let formData = new FormData();
      files.forEach((file) => {
        formData.append("images", file);
      });

      if (files.length > 1 && batchAnalyze) {
        formData.append("batch_analyze", "true");
      }

      const response = await axios.post(
        `${API_BASE_URL}${API_VERSION}/upload`,
        formData,
      );
      const { task_ids } = response.data;

      setProcessingTasks((prev) => {
        const updated = new Map(prev);
        task_ids.forEach((taskId: string) => {
          updated.set(taskId, { task_id: taskId, status: "pending" });
        });
        return updated;
      });

      showNotification(
        `${files.length} image(s) uploaded and queued for processing!`,
        "info",
      );
      setFiles([]);
    } catch (error) {
      showNotification("Failed to upload images", "error");
    } finally {
      setUploadLoading(false);
    }
  };

  const handleSearch = async () => {
    if (!query.trim()) {
      showNotification("Please enter a search query", "error");
      return;
    }

    setLoading(true);
    try {
      const res = await axios.post(`${API_BASE_URL}${API_VERSION}/search`, {
        query,
        top_k: 5,
      });
      setResults(res.data);
    } catch (error) {
      showNotification("Search failed", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-col items-center min-h-screen bg-gray-900 text-white p-4 md:p-6">
      {/* Add the custom styles */}
      <style dangerouslySetInnerHTML={{ __html: scrollbarHideStyles }} />
      <div className="max-w-4xl w-full">
        <h1 className="text-3xl md:text-4xl font-bold text-center my-8">
          🖼️ Image Analyzer + Vector DB
        </h1>

        {notification && (
          <div
            className={`mb-6 p-3 rounded text-center ${
              notification.type === "success"
                ? "bg-green-600"
                : notification.type === "error"
                  ? "bg-red-600"
                  : "bg-blue-600"
            }`}
          >
            {notification.message}
          </div>
        )}

        <div className="bg-gray-800 rounded-lg p-5 mb-6 shadow-lg">
          <h2 className="text-xl font-semibold mb-4">
            Upload Images (Up to 5)
          </h2>
          <div className="flex flex-col gap-3">
            <div className="flex-grow">
              <label className="block p-3 rounded border border-gray-600 bg-gray-700 text-white cursor-pointer hover:bg-gray-600 transition-colors">
                {files.length > 0
                  ? `${files.length} image(s) selected`
                  : "Select up to 5 images"}
                <input
                  className="hidden"
                  type="file"
                  accept="image/*"
                  multiple
                  onChange={(e) => {
                    const fileList = e.target.files;
                    if (fileList) {
                      const fileArray = Array.from(fileList).slice(0, 5);
                      setFiles(fileArray);
                    }
                  }}
                />
              </label>
            </div>
            {files.length > 0 && (
              <div className="flex flex-wrap gap-2 mt-2">
                {files.length > 1 && (
                  <div className="mt-3 flex items-center">
                    <input
                      type="checkbox"
                      id="batch-analyze"
                      checked={batchAnalyze}
                      onChange={(e) => setBatchAnalyze(e.target.checked)}
                      className="h-4 w-4 rounded border-gray-600 bg-gray-700 text-blue-600 focus:ring-blue-500"
                    />
                    <label htmlFor="batch-analyze" className="ml-2 text-sm">
                      Process images as a batch (analyze together)
                    </label>
                  </div>
                )}
                {files.map((file, index) => (
                  <div
                    key={index}
                    className="flex items-center bg-gray-700 rounded px-3 py-1"
                  >
                    <span className="text-sm truncate max-w-[150px]">
                      {file.name}
                    </span>
                    <button
                      onClick={() => {
                        setFiles(files.filter((_, i) => i !== index));
                      }}
                      className="ml-2 text-red-400 hover:text-red-300"
                    >
                      ✕
                    </button>
                  </div>
                ))}
              </div>
            )}
            <div className="mt-3">
              <button
                onClick={uploadImage}
                disabled={uploadLoading || files.length === 0}
                className={`w-full px-5 py-3 bg-blue-600 rounded-md font-medium transition-colors ${
                  uploadLoading || files.length === 0
                    ? "opacity-50 cursor-not-allowed"
                    : "hover:bg-blue-700"
                }`}
              >
                {uploadLoading
                  ? "Uploading..."
                  : `Upload ${files.length} Image(s)`}
              </button>
            </div>
          </div>
        </div>

        {processingTasks.size > 0 && (
          <div className="bg-gray-800 rounded-lg p-5 mb-6 shadow-lg">
            <h2 className="text-xl font-semibold mb-4">Processing Images</h2>
            <div className="space-y-3">
              {Array.from(processingTasks.entries()).map(([taskId, task]) => (
                <div
                  key={taskId}
                  className="p-3 rounded bg-gray-700 flex flex-col"
                >
                  <div className="flex justify-between items-center">
                    <div className="flex items-center">
                      {task.status === "pending" ||
                      task.status === "processing" ? (
                        <div className="mr-3 h-4 w-4 border-2 border-t-blue-500 border-r-transparent border-b-transparent border-l-transparent rounded-full animate-spin" />
                      ) : task.status === "completed" ? (
                        <svg
                          className="mr-3 h-5 w-5 text-green-500"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M5 13l4 4L19 7"
                          />
                        </svg>
                      ) : (
                        <svg
                          className="mr-3 h-5 w-5 text-red-500"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M6 18L18 6M6 6l12 12"
                          />
                        </svg>
                      )}
                      <span>Task {taskId.substring(0, 8)}...</span>
                      {task.result?.is_batch && (
                        <span className="ml-2 bg-indigo-700 text-xs px-2 py-1 rounded">
                          Batch
                        </span>
                      )}
                    </div>
                    <span
                      className="px-2 py-1 text-xs rounded capitalize"
                      style={{
                        backgroundColor:
                          task.status === "completed"
                            ? "rgba(74, 222, 128, 0.2)"
                            : task.status === "failed"
                              ? "rgba(248, 113, 113, 0.2)"
                              : "rgba(96, 165, 250, 0.2)",
                        color:
                          task.status === "completed"
                            ? "rgb(74, 222, 128)"
                            : task.status === "failed"
                              ? "rgb(248, 113, 113)"
                              : "rgb(96, 165, 250)",
                      }}
                    >
                      {task.status}
                    </span>
                  </div>

                  {/* Image previews for tasks */}
                  {task.result?.batch_paths &&
                    task.result.batch_paths.length > 0 && (
                      <div className="mt-3 grid grid-cols-3 gap-2">
                        {task.result.batch_paths.map(
                          (path: string, idx: number) => (
                            <img
                              key={idx}
                              src={`${API_BASE_URL}/${path}`}
                              alt={`Batch image ${idx + 1}`}
                              className="h-16 w-16 object-cover rounded"
                            />
                          ),
                        )}
                      </div>
                    )}

                  {/* For single image task that's completed */}
                  {task.status === "completed" &&
                    task.result?.file_path &&
                    !task.result?.is_batch && (
                      <div className="mt-3">
                        <img
                          src={`${API_BASE_URL}/${task.result.file_path}`}
                          alt="Processed image"
                          className="h-16 w-16 object-cover rounded"
                        />
                      </div>
                    )}
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="bg-gray-800 rounded-lg p-5 mb-6 shadow-lg">
          <h2 className="text-xl font-semibold mb-4">Search Images</h2>
          <div className="flex flex-col sm:flex-row gap-3">
            <input
              type="text"
              className="flex-grow p-3 rounded-md border border-gray-600 bg-gray-700 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="Describe what you're looking for..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSearch()}
            />
            <button
              onClick={handleSearch}
              disabled={loading}
              className={`px-5 py-3 bg-green-600 rounded-md font-medium transition-colors ${
                loading ? "opacity-50 cursor-not-allowed" : "hover:bg-green-700"
              }`}
            >
              {loading ? "Searching..." : "Search"}
            </button>
          </div>
        </div>

        {loading && (
          <div className="text-center py-10">
            <div className="inline-block h-8 w-8 border-4 border-t-blue-500 border-r-transparent border-b-transparent border-l-transparent rounded-full animate-spin"></div>
            <p className="mt-2">Searching...</p>
          </div>
        )}

        {results.length > 0 && (
          <div>
            <h2 className="text-2xl font-semibold mb-4">Results</h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {results.map((img, i) => (
                <div
                  key={i}
                  className="bg-gray-800 rounded-lg overflow-hidden shadow-lg hover:shadow-xl transition-shadow"
                >
                  {img.is_batch &&
                  img.batch_paths &&
                  img.batch_paths.length > 0 ? (
                    <div className="relative">
                      {/* Batch indicator */}
                      <div className="absolute top-2 right-2 bg-indigo-600 text-white text-xs px-2 py-1 rounded-full z-10">
                        Batch ({img.batch_paths.length} images)
                      </div>

                      {/* Image carousel for batch */}
                      <div className="flex overflow-x-auto snap-x scrollbar-hide">
                        {img.batch_paths.map((path, idx) => (
                          <div
                            key={idx}
                            className="snap-start w-full h-64 flex-shrink-0"
                          >
                            <img
                              src={`${API_BASE_URL}/${path}`}
                              alt={`Batch image ${idx + 1}`}
                              className="w-full h-64 object-cover object-center"
                            />
                          </div>
                        ))}
                      </div>

                      {/* Thumbnail navigation below */}
                      <div className="flex justify-center mt-2 px-4">
                        {img.batch_paths.map((path, idx) => (
                          <img
                            key={idx}
                            src={`${API_BASE_URL}/${path}`}
                            alt={`Thumbnail ${idx + 1}`}
                            className="w-12 h-12 object-cover rounded mx-1 cursor-pointer border-2 border-transparent hover:border-blue-500"
                            onClick={() => {
                              // Find the parent scrollable container and scroll to this image
                              const container = document.querySelector(
                                `div[key="${i}"] .flex.overflow-x-auto`,
                              );
                              if (container) {
                                const children =
                                  container.querySelectorAll(".snap-start");
                                if (children[idx]) {
                                  children[idx].scrollIntoView({
                                    behavior: "smooth",
                                    inline: "start",
                                  });
                                }
                              }
                            }}
                          />
                        ))}
                      </div>
                    </div>
                  ) : (
                    <img
                      src={`${API_BASE_URL}/${img.file_path}`}
                      alt="Search result"
                      className="w-full h-64 object-cover object-center"
                    />
                  )}
                  <div className="p-4">
                    <Markdown>{img.text}</Markdown>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {!loading && query && results.length === 0 && (
          <div className="text-center py-8 bg-gray-800 rounded-lg my-6">
            <p className="text-gray-400">No results found for your query</p>
          </div>
        )}
      </div>
    </div>
  );
}
