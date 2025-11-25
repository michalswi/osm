package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/michalswi/osm/utils"
)

var logger = log.New(os.Stdout, "osm: ", log.LstdFlags|log.Lshortfile)
var port = utils.GetEnv("SERVER_PORT", "5050")
var proxyStr = os.Getenv("PROXY_ADDR")

var (
	sourceJson = "source/locations.json"

	logMutex     sync.Mutex
	logPath      string
	ProxyClient  *http.Client
	proxyEnabled bool

	locationsCache      []ClientLocation
	locationsCacheMu    sync.RWMutex
	locationsCacheTTL   = 3 * time.Second
	locationsCacheStamp time.Time
)

var tpl = template.Must(template.New("page").Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>osm</title>
    <link rel="icon" href="web/pepe.png" type="image/png" sizes="16x16">
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="stylesheet" href="https://unpkg.com/leaflet/dist/leaflet.css">
    <script src="https://unpkg.com/leaflet/dist/leaflet.js"></script>
    <style>
        body { margin:0; font-family:Arial, sans-serif; }
        body.light { background:#f5f5f5; color:#111; }
        body.dark  { background:#2f2f2f; color:#eee; }

        #layout { display:flex; height:100vh; width:100vw; }

        /* LEFT SIDEBAR */
        #sidebar {
            width:300px;
            background:#20262c;
            color:#e2e6ea;
            display:flex;
            flex-direction:column;
            padding:12px;
            box-sizing:border-box;
            gap:14px;
        }
        body.light #sidebar { background:#ffffff; color:#222; }

        h2 { margin:0 0 6px; font-size:15px; font-weight:600; }
        .block { border:1px solid #313b44; border-radius:6px; padding:10px; }
        body.light .block { border-color:#d9d9d9; }

        input, select, button {
            font-size:13px; padding:6px 8px;
            border:1px solid #495661; border-radius:4px;
            background:#2d353c; color:#e2e6ea;
        }
        input:focus, select:focus { outline:2px solid #4d92ff; }
        button { cursor:pointer; }
        body.light input, body.light select, body.light button {
            background:#fafafa; color:#111; border-color:#c3c7cb;
        }
        button:hover { background:#3a444d; }
        body.light button:hover { background:#ececec; }

        #map-type { width:100%; }
        #github-btn svg { width:18px; height:18px; }
        .row { display:flex; gap:8px; }
        .stretch { width:100%; }
        .col { display:flex; flex-direction:column; gap:8px; }

        /* MAP */
        #map-wrap { flex:1; display:flex; }
        #map { flex:1; min-height:0; }

        /* Share URL */
        .share-url-block .row { align-items:stretch; }
        .share-url-block input { flex:1; min-width:0; }
        .share-url-block button { width:110px; }

        #footer { 
            margin-top:auto;
            text-align:center;
            font-size:11px;
            opacity:.6;
        }
    </style>
</head>
<body class="light">
<div id="layout">
    <div id="sidebar">
        <div class="row">
            <button id="theme-toggle" class="stretch" onclick="toggleTheme()">🌙 Dark Mode</button>
            <button id="github-btn" onclick="window.open('https://github.com/michalswi/osm','_blank','noopener')" aria-label="GitHub">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
                    <path d="M9 19c-5 1.5-5-2.5-7-3M17 22v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77 5.44 5.44 0 0 0 3.5 8.55c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/>
                </svg>
            </button>
        </div>

        <div class="block">
            <h2>Search</h2>
            <div class="row">
                <input id="search" type="text" placeholder="Wrocław, Poland">
                <button onclick="searchPlace()">Search</button>
            </div>
        </div>

        <div class="block">
            <h2>Enter Latitude & Longitude</h2>
            <div class="col">
                <input id="lat" type="number" step="any" value="{{.Lat}}" placeholder="Latitude">
                <input id="lon" type="number" step="any" value="{{.Lon}}" placeholder="Longitude">
            </div>
            <div class="row" style="margin-top:8px;">
                <button class="stretch" onclick="updateMap()">Show</button>
                <button class="stretch" onclick="findMyLocation()">Find Me</button>
            </div>
        </div>

        <div class="block">
            <h2>or Enter Coordinates (lat,lon)</h2>
            <div class="row">
                <input id="coord" type="text" placeholder="12.34,56.78">
                <button onclick="updateMapFromText()">Find</button>
            </div>
        </div>

        <div class="block">
            <h2>Map Type</h2>
            <select id="map-type" onchange="switchMapType()">
                <option value="street" selected>Street</option>
                <option value="satellite">Satellite</option>
                <option value="dark">Dark</option>
            </select>
        </div>

        <div class="block share-url-block">
            <h2>Share URL</h2>
            <div class="row">
                <input id="share-url" readonly placeholder="Click map or pin">
                <button onclick="copyShare()">Copy</button>
            </div>
        </div>

        <div id="footer">© 2049 michalswi</div>

    </div>

    <div id="map-wrap">
        <div id="map"></div>
    </div>
</div>
<script>
    var lat = parseFloat("{{.Lat}}");
    var lon = parseFloat("{{.Lon}}");

    var map = L.map('map', { 
        zoomControl:true  // Enable zoom controls 
    }).setView([lat, lon], 13);

    var lightTiles = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',{ attribution:'© OpenStreetMap contributors' });
    var satelliteTiles = L.tileLayer('https://mt1.google.com/vt/lyrs=s&x={x}&y={y}&z={z}',{ attribution:'© Google' });
    var darkTiles = L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',{ attribution:'© OpenStreetMap © CARTO' });
    
    var currentTiles = lightTiles;  // Default to street map
    currentTiles.addTo(map);
    
    var marker = L.marker([lat, lon], { draggable:true }).addTo(map)
        .bindPopup("Default Location")
        .openPopup();
    updateShareURL(lat, lon);

    marker.on('dragend', function(){
        var p = marker.getLatLng();
        document.getElementById('lat').value = p.lat.toFixed(6);
        document.getElementById('lon').value = p.lng.toFixed(6);
        updateShareURL(p.lat.toFixed(6), p.lng.toFixed(6));
    });
    marker.on('click', function(){
        var p = marker.getLatLng();
        updateShareURL(p.lat.toFixed(6), p.lng.toFixed(6));
    });

    // Add pins from locations.json
    var locations = {{.LocationsJSON}};
    locations.forEach(function(location){
        var detailsHTML = location.details;
        if (/^https?:\/\//i.test(detailsHTML)) {
            detailsHTML = '<a href="' + detailsHTML + '" target="_blank" rel="noopener">' + detailsHTML + '</a>';
        }
        var m = L.marker([location.lat, location.lon]).addTo(map)
            .bindPopup("as: " + location.as + "<br>asname: " + location.asname + "<br>details: " + detailsHTML);
        m.on('click', function(){
            updateShareURL(location.lat.toFixed(6), location.lon.toFixed(6));
        });
    });

    var dynamicMarkers = [];
    function clearDynamicMarkers(){ 
        dynamicMarkers.forEach(m=>map.removeLayer(m)); 
        dynamicMarkers=[];
    }

    function refreshLocations(){
        fetch('/api/locations')
          .then(r=>r.json())
          .then(list=>{
              clearDynamicMarkers();
              list.forEach(function(location){
                  var detailsHTML = location.details;
                  if (/^https?:\/\//i.test(detailsHTML)) {
                      detailsHTML = '<a href="' + detailsHTML + '" target="_blank" rel="noopener">' + detailsHTML + '</a>';
                  }
                  var dm = L.marker([location.lat, location.lon]).addTo(map)
                      .bindPopup("as: " + location.as + "<br>asname: " + location.asname + "<br>details: " + detailsHTML);
                  dm.on('click', function(){
                      updateShareURL(location.lat.toFixed(6), location.lon.toFixed(6));
                  });
                  dynamicMarkers.push(dm);
              });
          })
          .catch(()=>{});
    }

    setInterval(refreshLocations, 10000); // every 10s

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
        updateShareURL(clickedLat, clickedLon);
    });

    function searchPlace(){
        var q = document.getElementById('search').value.trim();
        if(!q) return alert("Enter a place.");
        fetch('https://nominatim.openstreetmap.org/search?format=json&q=' + encodeURIComponent(q))
          .then(r=>r.json())
          .then(data=>{
              if(!data.length) return alert("Not found.");
              var newLat = parseFloat(data[0].lat);
              var newLon = parseFloat(data[0].lon);
              document.getElementById('lat').value = newLat.toFixed(6);
              document.getElementById('lon').value = newLon.toFixed(6);
              map.setView([newLat, newLon], 13);
              marker.setLatLng([newLat, newLon]).bindPopup("Searched: " + data[0].display_name).openPopup();
              updateShareURL(newLat.toFixed(6), newLon.toFixed(6));
          })
          .catch(err=>alert("Search error: " + err.message));
    }

    function updateMap() {
        var newLat = parseFloat(document.getElementById('lat').value);
        var newLon = parseFloat(document.getElementById('lon').value);
        if (!isNaN(newLat) && !isNaN(newLon)) {
            map.setView([newLat, newLon], 13);
            marker.setLatLng([newLat, newLon])
                .bindPopup("New Location: " + newLat + ", " + newLon)
                .openPopup();
            updateShareURL(newLat.toFixed(6), newLon.toFixed(6));
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
                    updateShareURL(newLat.toFixed(6), newLon.toFixed(6));                            
                },
                function(error) {
                    alert("Unable to get your location: " + error.message);
                }
            );
        } else {
            alert("Geolocation is not supported by your browser.");
        }
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

    function toggleTheme(){
        var body = document.body;
        if(body.classList.contains('light')){
            body.classList.remove('light'); body.classList.add('dark');
            document.getElementById('theme-toggle').innerText='☀️ Light Mode';
        } else {
            body.classList.remove('dark'); body.classList.add('light');
            document.getElementById('theme-toggle').innerText='🌙 Dark Mode';
        }
    }

    function updateShareURL(latVal, lonVal){
        document.getElementById('share-url').value = location.origin + "?lat=" + latVal + "&lon=" + lonVal;
    }

    function copyShare(){
        var el = document.getElementById('share-url');
        if(!el.value) return;
        navigator.clipboard.writeText(el.value);
    }

</script>
</body>
</html>
`))
