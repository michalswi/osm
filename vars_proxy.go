package main

import (
	"text/template"
)

var tpl_proxy = template.Must(template.New("page").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>osm</title>
    <link rel="icon" href="web/pepe.png" type="image/png" sizes="16x16">
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
            /* remove absolute positioning from individual buttons */
            position: relative;
        }
        #github-btn {
            position: relative;
            padding: 8px;
            cursor: pointer;
            background: none;
            border: 1px solid #777;
            display: inline-flex;
            align-items: center;
            gap: 6px;
        }
        #github-btn svg {
            width: 20px;
            height: 20px;
        }
        #top-buttons {
            position: absolute;
            top: 10px;
            right: 10px;
            display: flex;
            flex-direction: column;
            gap: 8px;
            z-index: 500;
        }        
    </style>
</head>
<body class="light">

    <div id="top-buttons">
        <button id="theme-toggle" onclick="toggleTheme()">🌙 Dark Mode</button>
        <button id="github-btn" onclick="window.open('https://github.com/michalswi/osm','_blank','noopener')">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24"
                 fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"
                 stroke-linejoin="round" class="feather feather-github">
                <path d="M9 19c-5 1.5-5-2.5-7-3
                         M17 22v-3.87a3.37 3.37 0 0 0-.94-2.61
                         c3.14-.35 6.44-1.54 6.44-7
                         A5.44 5.44 0 0 0 20 4.77
                         A5.07 5.07 0 0 0 19.91 1
                         S18.73.65 16 2.48
                         a13.38 13.38 0 0 0-7 0
                         C6.27.65 5.09 1 5.09 1
                         A5.07 5.07 0 0 0 5 4.77
                         A5.44 5.44 0 0 0 3.5 8.55
                         c0 5.42 3.3 6.61 6.44 7
                         A3.37 3.37 0 0 0 9 18.13V22"/>
            </svg>
            GitHub
        </button>        
    </div>

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

		var lightTiles = L.tileLayer('/proxy/tiles/osm/{z}/{x}/{y}.png', {
			attribution: '© OpenStreetMap contributors'
		});

		var satelliteTiles = L.tileLayer('/proxy/tiles/google/{z}/{x}/{y}', {
			attribution: '© Google Maps'
		});

		var darkTiles = L.tileLayer('/proxy/tiles/carto/{z}/{x}/{y}.png', {
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
            var detailsHTML = location.details;
            if (/^https?:\/\//i.test(detailsHTML)) {
                detailsHTML = '<a href="' + detailsHTML + '" target="_blank" rel="noopener">' + detailsHTML + '</a>';
            }
            L.marker([location.lat, location.lon]).addTo(map)
                .bindPopup("as: " + location.as + "<br>asname: " + location.asname + "<br>details: " + detailsHTML);
        });

        var dynamicMarkers = [];
        function clearDynamicMarkers() {
            dynamicMarkers.forEach(m => map.removeLayer(m));
            dynamicMarkers = [];
        }

        function refreshLocations() {
            fetch('/api/locations')
              .then(r => r.json())
              .then(list => {
                  clearDynamicMarkers();
                  list.forEach(function(location) {
                      var detailsHTML = location.details;
                      if (/^https?:\/\//i.test(detailsHTML)) {
                          detailsHTML = '<a href="' + detailsHTML + '" target="_blank" rel="noopener">' + detailsHTML + '</a>';
                      }
                      var m = L.marker([location.lat, location.lon]).addTo(map)
                        .bindPopup("as: " + location.as + "<br>asname: " + location.asname + "<br>details: " + detailsHTML);
                      dynamicMarkers.push(m);
                  });
              })
              .catch(err => console.log('locations refresh error', err));
        }

        // initial async refresh (optional overrides embedded set)
        setInterval(refreshLocations, 10000); // every 10s

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

            fetch('/proxy/nominatim?q=' + encodeURIComponent(query))
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
