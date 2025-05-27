package main

import (
	"log"
	"os"
	"text/template"
)

var logger = log.New(os.Stdout, "osm: ", log.LstdFlags|log.Lshortfile)

var port = getEnv("SERVER_PORT", "5050")

var tpl = template.Must(template.New("page").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>osm</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="stylesheet" href="https://unpkg.com/leaflet/dist/leaflet.css" />
    <script src="https://unpkg.com/leaflet/dist/leaflet.js"></script>
    <style>
        body { 
            font-family: Arial, sans-serif; 
            text-align: center; 
            transition: background-color 0.3s, color 0.3s; 
        }
        body.light { 
            background-color: #f0f0f0; 
            color: #000; 
        }
        body.dark { 
            background-color: #333; 
            color: #fff; 
        }
        body.dark input, body.dark select, body.dark button {
            background-color: #555;
            color: #fff;
            border: 1px solid #777;
        }
        body.dark button:hover {
            background-color: #666;
        }
        #map { 
            height: 70vh; 
            margin-top: 10px; 
        }
        .input-container { 
            margin: 10px; 
        }
        input { 
            padding: 8px; 
            margin: 5px; 
            width: 120px; 
            transition: background-color 0.3s, color 0.3s, border 0.3s; 
        }
        #search { 
            width: 200px; 
        }
        select { 
            padding: 8px; 
            margin: 15px; 
            transition: background-color 0.3s, color 0.3s, border 0.3s; 
        }
        button { 
            padding: 8px; 
            cursor: pointer; 
            transition: background-color 0.3s, color 0.3s, border 0.3s; 
        }
        #theme-toggle { 
            position: absolute; 
            top: 10px; 
            right: 10px; 
        }
    </style>
</head>
<body class="light">
    <button id="theme-toggle" onclick="toggleTheme()">🌙 Dark Mode</button>

    <h2>Search for a Place</h2>
    <div class="input-container">
        <input type="text" id="search" placeholder="e.g. Wrocław, Poland">
        <button onclick="searchPlace()">Search</button>
    </div>

    <h2>Enter Latitude & Longitude</h2>
    <div class="input-container">
        <input type="number" id="lat" placeholder="Latitude" step="any" value="{{.Lat}}">
        <input type="number" id="lon" placeholder="Longitude" step="any" value="{{.Lon}}">
        <button onclick="updateMap()">Show Location</button>
    </div>

    <h2>or Enter Coordinates (lat,lon)</h2>
    <div class="input-container">
        <input type="text" id="coord" placeholder="e.g. 12.34,56.78">
        <button onclick="updateMapFromText()">Find Location</button>
    </div>

    <h2>or Find Your Current Location</h2>
    <div class="input-container">
        <button onclick="findMyLocation()">Find Me</button>
    </div>

    <h2>Map Type</h2>
    <div class="input-container">
        <select id="map-type" onchange="switchMapType()">
            <option value="street">Street Map</option>
            <option value="satellite">Satellite</option>
            <option value="dark">Dark Map</option>
        </select>
    </div>

    <h3>Click on the map to get coordinates</h3>

    <div id="map"></div>

    <script>
        var lat = parseFloat("{{.Lat}}");
        var lon = parseFloat("{{.Lon}}");

        var map = L.map('map', {
            zoomControl: true // Enable zoom controls
        }).setView([lat, lon], 13);

        var lightTiles = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            attribution: '© OpenStreetMap contributors'
        });

        var satelliteTiles = L.tileLayer('https://mt1.google.com/vt/lyrs=s&x={x}&y={y}&z={z}', {
            attribution: '© Google Maps'
        });

        var darkTiles = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
            attribution: '© OpenStreetMap contributors © CARTO'
        });

        var currentTiles = lightTiles; // Default to street map
        currentTiles.addTo(map);

        var marker = L.marker([lat, lon], { draggable: true }).addTo(map)
            .bindPopup("Default Location")
            .openPopup();

        // Add pins from locations.json
        var locations = {{.LocationsJSON}};
        locations.forEach(function(location) {
            L.marker([location.lat, location.lon]).addTo(map)
                .bindPopup("as: " + location.as + "<br>asname: " + location.asname);
        });

        function updateMap() {
            var newLat = parseFloat(document.getElementById('lat').value);
            var newLon = parseFloat(document.getElementById('lon').value);

            if (!isNaN(newLat) && !isNaN(newLon)) {
                map.setView([newLat, newLon], 13);
                marker.setLatLng([newLat, newLon])
                    .bindPopup("New Location: " + newLat + ", " + newLon)
                    .openPopup();
            } else {
                alert("Please enter valid latitude and longitude values.");
            }
        }

        function updateMapFromText() {
            var input = document.getElementById('coord').value.trim();
            var parts = input.split(",");

            if (parts.length === 2) {
                var newLat = parseFloat(parts[0]);
                var newLon = parseFloat(parts[1]);

                if (!isNaN(newLat) && !isNaN(newLon)) {
                    document.getElementById('lat').value = newLat;
                    document.getElementById('lon').value = newLon;
                    updateMap();
                } else {
                    alert("Invalid format. Use lat,lon (e.g. 12.34,56.78).");
                }
            } else {
                alert("Invalid format. Use lat,lon (e.g. 12.34,56.78).");
            }
        }

        // Click event to get coordinates
        map.on('click', function(e) {
            var clickedLat = e.latlng.lat.toFixed(6);
            var clickedLon = e.latlng.lng.toFixed(6);

            // Update input fields
            document.getElementById('lat').value = clickedLat;
            document.getElementById('lon').value = clickedLon;

            // Move marker to clicked location
            marker.setLatLng([clickedLat, clickedLon])
                .bindPopup("Clicked Location: " + clickedLat + ", " + clickedLon)
                .openPopup();
        });

        // Find user's current location
        function findMyLocation() {
            if (navigator.geolocation) {
                navigator.geolocation.getCurrentPosition(
                    function(position) {
                        var newLat = position.coords.latitude;
                        var newLon = position.coords.longitude;

                        // Update input fields
                        document.getElementById('lat').value = newLat.toFixed(6);
                        document.getElementById('lon').value = newLon.toFixed(6);

                        // Update map and marker
                        map.setView([newLat, newLon], 13);
                        marker.setLatLng([newLat, newLon])
                            .bindPopup("Your Location: " + newLat.toFixed(6) + "," + newLon.toFixed(6))
                            .openPopup();
                    },
                    function(error) {
                        alert("Unable to get your location: " + error.message);
                    }
                );
            } else {
                alert("Geolocation is not supported by your browser.");
            }
        }

        // Search for a place using Nominatim API
        function searchPlace() {
            var query = document.getElementById('search').value.trim();
            if (query === "") {
                alert("Please enter a place to search for.");
                return;
            }

            fetch('https://nominatim.openstreetmap.org/search?format=json&q=' + encodeURIComponent(query))
                .then(response => response.json())
                .then(data => {
                    if (data.length > 0) {
                        var newLat = parseFloat(data[0].lat);
                        var newLon = parseFloat(data[0].lon);

                        // Update input fields
                        document.getElementById('lat').value = newLat.toFixed(6);
                        document.getElementById('lon').value = newLon.toFixed(6);

                        // Update map and marker
                        map.setView([newLat, newLon], 13);
                        marker.setLatLng([newLat, newLon])
                            .bindPopup("Searched Location: " + data[0].display_name)
                            .openPopup();
                    } else {
                        alert("Place not found.");
                    }
                })
                .catch(error => {
                    alert("Error searching for place: " + error.message);
                });
        }

        // Switch map type
        function switchMapType() {
            var mapType = document.getElementById('map-type').value;
            currentTiles.remove();
            if (mapType === 'street') {
                currentTiles = lightTiles;
            } else if (mapType === 'satellite') {
                currentTiles = satelliteTiles;
            } else if (mapType === 'dark') {
                currentTiles = darkTiles;
            }
            currentTiles.addTo(map);
        }

        // Dark mode toggle (affects UI only, not the map)
        function toggleTheme() {
            var body = document.body;
            if (body.classList.contains('light')) {
                body.classList.remove('light');
                body.classList.add('dark');
                document.getElementById('theme-toggle').innerText = '☀️ Light Mode';
            } else {
                body.classList.remove('dark');
                body.classList.add('light');
                document.getElementById('theme-toggle').innerText = '🌙 Dark Mode';
            }
        }
    </script>
    <div class="container">
      <hr/>
      <p>
        Copyright © 2049
        michalswi<br>
      </p>
    </div>
</body>
</html>
`))
