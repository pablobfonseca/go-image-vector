"use client";
import axios from "axios";
import { useState } from "react";
import Markdown from "react-markdown";

type Embedding = {
  id: number;
  file_path: string;
  text: string;
  embedding: Array<number>;
};

export default function Home() {
  const [file, setFile] = useState<File | null>(null);
  const [query, setQuery] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(false);
  const [uploadLoading, setUploadLoading] = useState<boolean>(false);
  const [results, setResults] = useState<Array<Embedding>>([]);
  const [notification, setNotification] = useState<{
    message: string;
    type: "success" | "error";
  } | null>(null);

  const showNotification = (message: string, type: "success" | "error") => {
    setNotification({ message, type });
    setTimeout(() => setNotification(null), 3000);
  };

  const uploadImage = async () => {
    if (!file) {
      showNotification("Please select a file first", "error");
      return;
    }

    setUploadLoading(true);
    try {
      let formData = new FormData();
      formData.append("image", file);

      await axios.post("http://localhost:8080/upload", formData);
      showNotification("Image uploaded successfully!", "success");
      setFile(null);
    } catch (error) {
      showNotification("Failed to upload image", "error");
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
      const res = await axios.post("http://localhost:8080/search", {
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
      <div className="max-w-4xl w-full">
        <h1 className="text-3xl md:text-4xl font-bold text-center my-8">
          🖼️ Image Analyzer + Vector DB
        </h1>

        {notification && (
          <div
            className={`mb-6 p-3 rounded text-center ${
              notification.type === "success" ? "bg-green-600" : "bg-red-600"
            }`}
          >
            {notification.message}
          </div>
        )}

        <div className="bg-gray-800 rounded-lg p-5 mb-6 shadow-lg">
          <h2 className="text-xl font-semibold mb-4">Upload Image</h2>
          <div className="flex flex-col sm:flex-row gap-3">
            <div className="flex-grow">
              <label className="block p-3 rounded border border-gray-600 bg-gray-700 text-white cursor-pointer hover:bg-gray-600 transition-colors">
                {file ? file.name : "Select an image file"}
                <input
                  className="hidden"
                  type="file"
                  accept="image/*"
                  onChange={(e) => e.target.files && setFile(e.target.files[0])}
                />
              </label>
            </div>
            <button
              onClick={uploadImage}
              disabled={uploadLoading}
              className={`px-5 py-3 bg-blue-600 rounded-md font-medium transition-colors ${
                uploadLoading
                  ? "opacity-50 cursor-not-allowed"
                  : "hover:bg-blue-700"
              }`}
            >
              {uploadLoading ? "Uploading..." : "Upload"}
            </button>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-5 mb-6 shadow-lg">
          <h2 className="text-xl font-semibold mb-4">Search Images</h2>
          <div className="flex flex-col sm:flex-row gap-3">
            <input
              type="text"
              className="flex-grow p-3 rounded-md border border-gray-600 bg-gray-700 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="Describe what you're looking for..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyPress={(e) => e.key === "Enter" && handleSearch()}
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
                  <img
                    src={`http://localhost:8080/${img.file_path}`}
                    alt="Search result"
                    className="w-full h-64 object-cover object-center"
                  />
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
